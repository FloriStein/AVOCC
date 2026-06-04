// Package recording implements the Session Recording interface (BE-07, ADR-005/016).
// All recording is done against the SessionRecorder interface — the storage backend
// is replaceable via a future ADR (ADR-005 Folge: DB/Files/Object Storage).
package recording

import "time"

// Entry is a single recorded event within a session (ADR-005).
type Entry struct {
	SessionID   string
	EventID     string
	VehicleID   string
	OperatorID  string
	Timestamp   time.Time
	EntryType   string // "control", "safety", "state"
	CommandType string // for control entries
	Value       float32
	SystemState string // for state entries
	CtrlState   string
	EventType   string // for safety entries
	Reason      string
}

// SessionRecorder is the recording interface (ADR-005).
// Implementations: MemoryRecorder (Sprint 4), future storage adapters.
type SessionRecorder interface {
	StartSession(sessionID, vehicleID, operatorID string)
	EndSession(sessionID string)
	RecordControlEvent(sessionID, eventID, vehicleID, operatorID, commandType string, value float32)
	RecordStateSnapshot(sessionID, eventID, vehicleID, operatorID, systemState, ctrlState string)
	RecordSafetyEvent(sessionID, eventID, vehicleID, operatorID, eventType, reason string)
	GetEntries(sessionID string) []Entry
}
