// Package statemachine implements the 4-Layer State Machine (ADR-011).
// SYSTEM STATE is the Safety Truth — all other states depend on it.
// MEDIA STATE can never trigger SAFE_MODE (ADR-009 Invariant 1).
package statemachine

import (
	"log"
	"sync"
)

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

// validSystemTransitions defines allowed state transitions.
// SAFE_MODE is reachable from any non-idle state (CRITICAL can occur at any time).
var validSystemTransitions = map[SystemState][]SystemState{
	StateIdle:          {StateConnecting},
	StateConnecting:    {StateAuthenticated, StateSafeMode},
	StateAuthenticated: {StateConnected, StateSafeMode},
	StateConnected:     {StateDegraded, StateSafeMode},
	StateDegraded:      {StateConnected, StateSafeMode},
	StateSafeMode:      {StateRecovering},
	StateRecovering:    {StateAuthenticated, StateSafeMode},
}

// Machine holds all 4 orthogonal state machines.
type Machine struct {
	mu       sync.RWMutex
	System   SystemState
	Control  ControlState
	Media    MediaState
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

// CanTransitionTo returns true if transitioning from current to next system state is valid.
func (m *Machine) CanTransitionTo(next SystemState) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return isValidTransition(m.System, next)
}

func isValidTransition(current, next SystemState) bool {
	allowed, ok := validSystemTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// TransitionSystem sets the SYSTEM STATE and enforces dependent CONTROL STATE rules.
// Invalid transitions are logged and rejected — system stays in current state.
// SAFE_MODE → CONTROL_BLOCKED (ADR-011).
func (m *Machine) TransitionSystem(next SystemState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !isValidTransition(m.System, next) {
		log.Printf("[STATE] invalid transition rejected: %s → %s", m.System, next)
		return
	}

	log.Printf("[STATE] SYSTEM: %s → %s", m.System, next)
	m.System = next

	switch next {
	case StateSafeMode:
		m.Control = ControlBlocked
	case StateConnected:
		m.Control = ControlActive
	case StateAuthenticated:
		if m.Control == ControlBlocked || m.Control == ControlRecovering {
			m.Control = ControlInit
		}
	case StateRecovering:
		m.Control = ControlRecovering
	case StateDegraded:
		// Control remains active during DEGRADED — video loss never blocks control (ADR-011)
	}
}

// TransitionToConnected atomically moves AUTHENTICATED → CONNECTED and activates control.
// Returns false if the precondition (must be in AUTHENTICATED) is not met.
func (m *Machine) TransitionToConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.System != StateAuthenticated {
		log.Printf("[STATE] TransitionToConnected rejected: must be AUTHENTICATED, was %s", m.System)
		return false
	}
	log.Printf("[STATE] SYSTEM: %s → %s", m.System, StateConnected)
	m.System = StateConnected
	m.Control = ControlActive
	return true
}

// TransitionMedia updates media state and maps MEDIA_FAILED/DEGRADED → SYSTEM DEGRADED.
// MEDIA events NEVER trigger SAFE_MODE (ADR-009 Invariant 1).
func (m *Machine) TransitionMedia(next MediaState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Media = next
	if (next == MediaFailed || next == MediaDegraded) && m.System == StateConnected {
		log.Printf("[STATE] MEDIA %s → SYSTEM DEGRADED (Invariant 1: never SAFE_MODE)", next)
		m.System = StateDegraded
	}
}

// TransitionOperator updates operator state and enforces NO_OPERATOR → SAFE_MODE (ADR-011).
func (m *Machine) TransitionOperator(next OperatorState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Operator = next
	if next == OpNoOperator && (m.System == StateConnected || m.System == StateDegraded) {
		log.Printf("[STATE] OPERATOR → NO_OPERATOR → SAFE_MODE (no active operator)")
		m.System = StateSafeMode
		m.Control = ControlBlocked
	}
}
