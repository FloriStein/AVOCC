// Package statemachine implements the 4-Layer State Machine (ADR-011).
// SYSTEM STATE is the Safety Truth — all other states depend on it.
// MEDIA STATE can never trigger SAFE_MODE (ADR-009 Invariant 1).
package statemachine

import "sync"

// SystemState represents the master safety state (ADR-011).
type SystemState string

const (
	StateIdle          SystemState = "IDLE"
	StateConnecting    SystemState = "CONNECTING"
	StateAuthenticated SystemState = "AUTHENTICATED"
	StateConnected     SystemState = "CONNECTED"
	StateDegraded      SystemState = "DEGRADED"
	StateSafeMode      SystemState = "SAFE_MODE"
	StateRecovering    SystemState = "RECOVERING"
)

// ControlState represents the command flow state (ADR-011).
type ControlState string

const (
	ControlInit       ControlState = "CONTROL_INIT"
	ControlActive     ControlState = "CONTROL_ACTIVE"
	ControlBlocked    ControlState = "CONTROL_BLOCKED" // enforced during SAFE_MODE
	ControlLost       ControlState = "CONTROL_LOST"
	ControlRecovering ControlState = "CONTROL_RECOVERING"
)

// MediaState represents WebRTC video health (ADR-011).
// MEDIA_FAILED maps to SYSTEM DEGRADED — never SAFE_MODE (ADR-009 Invariant 1).
type MediaState string

const (
	MediaInit        MediaState = "MEDIA_INIT"
	MediaNegotiating MediaState = "MEDIA_NEGOTIATING"
	MediaConnected   MediaState = "MEDIA_CONNECTED"
	MediaDegraded    MediaState = "MEDIA_DEGRADED"
	MediaFailed      MediaState = "MEDIA_FAILED"
)

// OperatorState represents human governance (ADR-011).
type OperatorState string

const (
	OpNoOperator      OperatorState = "NO_OPERATOR"
	OpAssigned        OperatorState = "OPERATOR_ASSIGNED"
	OpActive          OperatorState = "ACTIVE_OPERATOR"
	OpHandoverPending OperatorState = "HANDOVER_PENDING"
	OpRecovering      OperatorState = "RECOVERING_OPERATOR"
)

// Machine holds all 4 orthogonal state machines.
type Machine struct {
	mu      sync.RWMutex
	System  SystemState
	Control ControlState
	Media   MediaState
	Operator OperatorState
}

func New() *Machine {
	return &Machine{
		System:   StateIdle,
		Control:  ControlInit,
		Media:    MediaInit,
		Operator: OpNoOperator,
	}
}

func (m *Machine) Get() (SystemState, ControlState, MediaState, OperatorState) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.System, m.Control, m.Media, m.Operator
}

// TransitionSystem sets the SYSTEM STATE and enforces CONTROL STATE rules.
// SAFE_MODE → CONTROL_BLOCKED (ADR-011).
func (m *Machine) TransitionSystem(next SystemState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.System = next
	switch next {
	case StateSafeMode:
		m.Control = ControlBlocked
	case StateConnected, StateAuthenticated:
		if m.Control == ControlBlocked {
			m.Control = ControlInit
		}
	case StateRecovering:
		m.Control = ControlRecovering
	}
}

// TransitionMedia updates media state and maps MEDIA_FAILED → DEGRADED.
// MEDIA events NEVER trigger SAFE_MODE (ADR-009 Invariant 1).
func (m *Machine) TransitionMedia(next MediaState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Media = next
	if (next == MediaFailed || next == MediaDegraded) && m.System == StateConnected {
		m.System = StateDegraded
	}
}

func (m *Machine) TransitionOperator(next OperatorState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Operator = next
	// NO_OPERATOR → SAFE_MODE (ADR-011)
	if next == OpNoOperator && (m.System == StateConnected || m.System == StateDegraded) {
		m.System = StateSafeMode
		m.Control = ControlBlocked
	}
}
