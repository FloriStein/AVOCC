// Package session implements the Session Manager — the Global Session Authority (GSA) of the
// Control Server (ADR-015). It is the single point responsible for session lifecycle,
// recovery checkpoints, and SFU event distribution.
package session

import (
	"sync"
	"time"

	"avoc/pkg/ulid"
)

// Session represents an active Control Session (ADR-015).
// Exactly: 1 Vehicle + 1 Active Operator + 1 Control Server instance.
type Session struct {
	ID           string    // ULID — root anchor, survives SAFE_MODE (ADR-016)
	VehicleID    string
	OperatorID   string
	OperatorRole string
	CreatedAt    time.Time
}

// RecoveryCheckpoint is saved on every SAFE_MODE entry (ADR-015).
// It is the basis for RECOVERING → CONNECTED re-activation.
type RecoveryCheckpoint struct {
	SessionID        string
	VehicleID        string
	OperatorID       string
	LastSystemState  string
	LastControlState string
	SafetyReason     string
	CheckpointTS     time.Time
}

// Manager is the Global Session Authority (GSA).
// It is the only component allowed to create, modify, and destroy sessions.
type Manager struct {
	mu           sync.RWMutex
	current      *Session
	checkpoint   *RecoveryCheckpoint
	sfuPublisher SFUPublisher
}

func NewManager(sfuPublisher SFUPublisher) *Manager {
	return &Manager{sfuPublisher: sfuPublisher}
}

// CreateSession generates a new Control Session at the AUTHENTICATED→CONNECTED boundary (ADR-016).
// The ULID is the session's root anchor — it survives SAFE_MODE.
func (m *Manager) CreateSession(vehicleID, operatorID, operatorRole string) Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Session{
		ID:           ulid.Generate(),
		VehicleID:    vehicleID,
		OperatorID:   operatorID,
		OperatorRole: operatorRole,
		CreatedAt:    time.Now(),
	}
	m.current = &s
	return s
}

// GetCurrentSession returns the active session. Returns false if no session is active.
func (m *Manager) GetCurrentSession() (Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return Session{}, false
	}
	return *m.current, true
}

// UpdateOperator replaces the active operator — used during Handover (ADR-011/015).
func (m *Manager) UpdateOperator(operatorID, operatorRole string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return
	}
	m.current.OperatorID = operatorID
	m.current.OperatorRole = operatorRole
}

// SaveCheckpoint freezes session state at SAFE_MODE entry (ADR-015).
// Recovery = neu-aktivierter Zustand unter gleicher Session-ID, kein Auto-Resume.
func (m *Manager) SaveCheckpoint(sysState, ctrlState, safetyReason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return
	}
	m.checkpoint = &RecoveryCheckpoint{
		SessionID:        m.current.ID,
		VehicleID:        m.current.VehicleID,
		OperatorID:       m.current.OperatorID,
		LastSystemState:  sysState,
		LastControlState: ctrlState,
		SafetyReason:     safetyReason,
		CheckpointTS:     time.Now(),
	}
}

// LoadCheckpoint returns the last saved recovery checkpoint, if any.
func (m *Manager) LoadCheckpoint() (*RecoveryCheckpoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.checkpoint == nil {
		return nil, false
	}
	cp := *m.checkpoint
	return &cp, true
}

// EndSession clears the active session (called on SESSION_ENDED).
func (m *Manager) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = nil
}

// PushSFUEvent sends a session event to the SFU asynchronously (ADR-015).
// The SFU consumes but never interprets — Dumb Media Router with State Subscription.
func (m *Manager) PushSFUEvent(eventType string) {
	m.mu.RLock()
	s := m.current
	m.mu.RUnlock()
	if s == nil || m.sfuPublisher == nil {
		return
	}
	go m.sfuPublisher.PublishSessionEvent(eventType, s.ID, s.OperatorID)
}
