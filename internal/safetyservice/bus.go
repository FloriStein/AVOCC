// Package safetyservice implements the Safety Event Bus (ADR-002).
// Interface is designed to be DDS-compatible for future replacement (ADR-002).
// Media events MUST NOT trigger SAFE_MODE — enforced via SafetyEventType enum (ADR-009 Invariant 2).
package safetyservice

import (
	"sync"
	"time"
)

// SafetyEventType maps to CRITICAL triggers (ADR-009).
// MEDIA_* types are intentionally absent — they never trigger SAFE_MODE.
type SafetyEventType string

const (
	EventEmergencyStop  SafetyEventType = "EMERGENCY_STOP"
	EventDeadmanTimeout SafetyEventType = "DEADMAN_TIMEOUT"
	EventACKTimeout     SafetyEventType = "ACK_TIMEOUT"
	EventWSDisconnect   SafetyEventType = "WS_DISCONNECT"
	EventNoOperator     SafetyEventType = "NO_OPERATOR"
	EventAuthInvalid    SafetyEventType = "AUTH_INVALID"
	EventSafetyBusDown  SafetyEventType = "SAFETY_BUS_DOWN"
)

type SafetyEvent struct {
	SessionID string
	EventID   string
	VehicleID string
	Type      SafetyEventType
	Reason    string
	Timestamp time.Time
}

type SafetyState struct {
	SafeMode  bool
	LastEvent SafetyEventType
	UpdatedAt time.Time
}

type SafetyEventHandler func(event SafetyEvent)

// Bus is the in-memory Safety Event Bus.
// Interface contract is DDS-compatible for future replacement.
type Bus struct {
	mu        sync.RWMutex
	state     SafetyState
	handlers  []SafetyEventHandler
}

func NewBus() *Bus {
	return &Bus{}
}

// PublishSafetyEvent publishes a safety event asynchronously (fire-and-forget — ADR-012b).
func (b *Bus) PublishSafetyEvent(event SafetyEvent) {
	b.mu.Lock()
	b.state = SafetyState{
		SafeMode:  true,
		LastEvent: event.Type,
		UpdatedAt: time.Now(),
	}
	handlers := make([]SafetyEventHandler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.Unlock()

	// Notify handlers asynchronously
	for _, h := range handlers {
		h := h
		go h(event)
	}
}

// TriggerEmergencyStop is a convenience method for the highest-priority safety event.
func (b *Bus) TriggerEmergencyStop(sessionID, vehicleID, reason string) {
	b.PublishSafetyEvent(SafetyEvent{
		SessionID: sessionID,
		VehicleID: vehicleID,
		Type:      EventEmergencyStop,
		Reason:    reason,
		Timestamp: time.Now(),
	})
}

// GetSafetyState returns the current safety state (thread-safe).
func (b *Bus) GetSafetyState() SafetyState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Subscribe registers a handler for safety events.
func (b *Bus) Subscribe(handler SafetyEventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

// Reset clears SAFE_MODE — only called after successful Recovery + Operator Ack (ADR-009).
func (b *Bus) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = SafetyState{}
}
