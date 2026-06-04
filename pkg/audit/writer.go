// Package audit provides guaranteed-persistence storage for safety events (ADR-018).
// Safety events must NEVER be lost — they write synchronously via WriteSync()
// with fsync before the SAFE_MODE state transition fires.
package audit

import "time"

// SafetyAuditEvent is a safety-critical event written before every SAFE_MODE transition.
type SafetyAuditEvent struct {
	EventID     string
	SessionID   string
	VehicleID   string
	OperatorID  string
	EventType   string
	Reason      string
	SystemState string
	CtrlState   string
	Data        string // JSON-encoded extra data (optional)
	Timestamp   time.Time
}

// AuditWriter persists safety events with a durability guarantee (ADR-018).
// WriteSync must complete before the SAFE_MODE transition fires.
// Implementations: SQLiteAuditWriter (production), NoopWriter (tests).
type AuditWriter interface {
	// WriteSync writes the event synchronously and fsyncs before returning.
	// Must be called BEFORE TransitionSystem(StateSafeMode).
	// A write error is logged but does NOT prevent the SAFE_MODE transition.
	WriteSync(event SafetyAuditEvent) error

	// QueryBySession returns all safety events for a session in timestamp order.
	QueryBySession(sessionID string) ([]SafetyAuditEvent, error)

	// Close releases resources (DB connection).
	Close() error
}
