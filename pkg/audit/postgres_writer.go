package audit

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

const schema = `
CREATE TABLE IF NOT EXISTS audit_events (
    id           SERIAL PRIMARY KEY,
    event_id     TEXT NOT NULL UNIQUE,
    session_id   TEXT NOT NULL,
    vehicle_id   TEXT NOT NULL,
    operator_id  TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    reason       TEXT,
    system_state TEXT,
    ctrl_state   TEXT,
    data         TEXT,
    timestamp    TEXT NOT NULL,
    written_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_session    ON audit_events(session_id);
CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp  ON audit_events(timestamp);
`

// PostgresAuditWriter persists safety events to PostgreSQL.
// WriteSync() relies on PostgreSQL's synchronous_commit=on (default) for WAL durability (ADR-023).
type PostgresAuditWriter struct {
	db *sql.DB
}

func NewPostgresAuditWriter(db *sql.DB) (*PostgresAuditWriter, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("audit: create schema: %w", err)
	}
	return &PostgresAuditWriter{db: db}, nil
}

// WriteSync persists the event and blocks until PostgreSQL COMMIT returns (WAL-durable by default).
func (w *PostgresAuditWriter) WriteSync(e SafetyAuditEvent) error {
	_, err := w.db.Exec(`
		INSERT INTO audit_events
			(event_id, session_id, vehicle_id, operator_id, event_type,
			 reason, system_state, ctrl_state, data, timestamp, written_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (event_id) DO NOTHING`,
		e.EventID, e.SessionID, e.VehicleID, e.OperatorID, e.EventType,
		e.Reason, e.SystemState, e.CtrlState, e.Data,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	return nil
}

// QueryBySession returns all safety events for a session, ordered by timestamp ascending.
func (w *PostgresAuditWriter) QueryBySession(sessionID string) ([]SafetyAuditEvent, error) {
	rows, err := w.db.Query(`
		SELECT event_id, session_id, vehicle_id, operator_id, event_type,
		       reason, system_state, ctrl_state, data, timestamp
		FROM audit_events
		WHERE session_id = $1
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

func (w *PostgresAuditWriter) Close() error {
	return w.db.Close()
}
