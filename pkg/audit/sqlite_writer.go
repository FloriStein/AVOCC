package audit

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Pure-Go SQLite driver — no CGO required (modernc.org/sqlite, ADR-018).
	// Run: go get modernc.org/sqlite
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS audit_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id    TEXT    NOT NULL UNIQUE,
    session_id  TEXT    NOT NULL,
    vehicle_id  TEXT    NOT NULL,
    operator_id TEXT    NOT NULL,
    event_type  TEXT    NOT NULL,
    reason      TEXT,
    system_state TEXT,
    ctrl_state  TEXT,
    data        TEXT,
    timestamp   TEXT    NOT NULL,
    written_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session    ON audit_events(session_id);
CREATE INDEX IF NOT EXISTS idx_event_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_timestamp  ON audit_events(timestamp);
`

// SQLiteAuditWriter persists safety events to a WAL-mode SQLite database.
// WriteSync() commits and fsyncs before returning — guaranteed durability (ADR-018).
type SQLiteAuditWriter struct {
	db *sql.DB
}

// NewSQLiteAuditWriter opens (or creates) the SQLite database at dbPath.
// Enables WAL journal mode for concurrent read access and crash safety.
func NewSQLiteAuditWriter(dbPath string) (*SQLiteAuditWriter, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("audit: create dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("audit: open db: %w", err)
	}

	// Single writer — serialise writes, allow concurrent readers.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("audit: enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("audit: set synchronous=FULL: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("audit: create schema: %w", err)
	}

	return &SQLiteAuditWriter{db: db}, nil
}

// WriteSync persists the event and issues a WAL checkpoint to fsync the data file.
// Blocks until the write is durable. Must be called before SAFE_MODE transition (ADR-018).
func (w *SQLiteAuditWriter) WriteSync(e SafetyAuditEvent) error {
	_, err := w.db.Exec(`
		INSERT OR IGNORE INTO audit_events
			(event_id, session_id, vehicle_id, operator_id, event_type,
			 reason, system_state, ctrl_state, data, timestamp, written_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.EventID, e.SessionID, e.VehicleID, e.OperatorID, e.EventType,
		e.Reason, e.SystemState, e.CtrlState, e.Data,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	// PRAGMA wal_checkpoint(FULL) flushes WAL to the main DB file — equivalent to fsync.
	_, err = w.db.Exec("PRAGMA wal_checkpoint(FULL)")
	return err
}

// QueryBySession returns all safety events for a session, ordered by timestamp ascending.
func (w *SQLiteAuditWriter) QueryBySession(sessionID string) ([]SafetyAuditEvent, error) {
	rows, err := w.db.Query(`
		SELECT event_id, session_id, vehicle_id, operator_id, event_type,
		       reason, system_state, ctrl_state, data, timestamp
		FROM audit_events
		WHERE session_id = ?
		ORDER BY timestamp ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("audit: query: %w", err)
	}
	defer rows.Close()

	var events []SafetyAuditEvent
	for rows.Next() {
		var e SafetyAuditEvent
		var ts string
		if err := rows.Scan(
			&e.EventID, &e.SessionID, &e.VehicleID, &e.OperatorID, &e.EventType,
			&e.Reason, &e.SystemState, &e.CtrlState, &e.Data, &ts,
		); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (w *SQLiteAuditWriter) Close() error {
	return w.db.Close()
}
