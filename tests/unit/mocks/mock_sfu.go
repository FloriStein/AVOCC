package mocks

import "sync"

// SFUEvent captures a single session event pushed to the SFU.
type SFUEvent struct {
	Type       string
	SessionID  string
	OperatorID string
}

// MockSFUPublisher records session events for test assertions.
// Implements internal/controlserver/session.SFUPublisher.
type MockSFUPublisher struct {
	mu     sync.Mutex
	events []SFUEvent
}

func (m *MockSFUPublisher) PublishSessionEvent(eventType, sessionID, operatorID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, SFUEvent{
		Type:       eventType,
		SessionID:  sessionID,
		OperatorID: operatorID,
	})
}

func (m *MockSFUPublisher) Events() []SFUEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SFUEvent, len(m.events))
	copy(out, m.events)
	return out
}

func (m *MockSFUPublisher) LastEventType() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) == 0 {
		return ""
	}
	return m.events[len(m.events)-1].Type
}

func (m *MockSFUPublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}
