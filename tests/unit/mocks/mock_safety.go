// Package mocks provides test doubles for Sprint 2 unit tests (ADR-006).
package mocks

import (
	"sync"

	"avoc/internal/safetyservice"
)

// MockSafetyPublisher records all published safety events for test assertions.
// Implements internal/controlserver/safety.Publisher.
type MockSafetyPublisher struct {
	mu     sync.Mutex
	events []safetyservice.SafetyEvent
	stops  int
}

func (m *MockSafetyPublisher) TriggerEmergencyStop(sessionID, vehicleID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops++
	m.events = append(m.events, safetyservice.SafetyEvent{
		SessionID: sessionID,
		VehicleID: vehicleID,
		Type:      safetyservice.EventEmergencyStop,
		Reason:    reason,
	})
}

func (m *MockSafetyPublisher) PublishEvent(event safetyservice.SafetyEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *MockSafetyPublisher) Events() []safetyservice.SafetyEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]safetyservice.SafetyEvent, len(m.events))
	copy(out, m.events)
	return out
}

func (m *MockSafetyPublisher) LastEventType() safetyservice.SafetyEventType {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) == 0 {
		return ""
	}
	return m.events[len(m.events)-1].Type
}

func (m *MockSafetyPublisher) EmergencyStopCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stops
}

func (m *MockSafetyPublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
	m.stops = 0
}
