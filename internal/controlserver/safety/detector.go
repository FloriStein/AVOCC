package safety

import (
	"log"
	"sync"
	"time"

	"avoc/internal/controlserver/statemachine"
	"avoc/internal/safetyservice"
)

const (
	DefaultDeadmanTimeout = 2 * time.Second
	DefaultACKTimeout     = 100 * time.Millisecond
)

// DeadmanWatchdog fires SAFE_MODE if Reset() is not called within Timeout.
// Reset() must be called on every DEADMAN_HOLD command from the operator (ADR-009).
// Loslassen = timeout = CRITICAL.
type DeadmanWatchdog struct {
	mu        sync.Mutex
	timer     *time.Timer
	timeout   time.Duration
	sm        *statemachine.Machine
	publisher Publisher
	sessionID string
	vehicleID string
	stopped   bool
}

func NewDeadmanWatchdog(timeout time.Duration, sm *statemachine.Machine, publisher Publisher) *DeadmanWatchdog {
	return &DeadmanWatchdog{
		timeout:   timeout,
		sm:        sm,
		publisher: publisher,
	}
}

// Start activates the watchdog for the given session. Call when entering CONTROL_ACTIVE.
func (w *DeadmanWatchdog) Start(sessionID, vehicleID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sessionID = sessionID
	w.vehicleID = vehicleID
	w.stopped = false
	w.timer = time.AfterFunc(w.timeout, w.fire)
	log.Printf("[DEADMAN] watchdog started (timeout=%s, session=%s)", w.timeout, sessionID)
}

// Reset restarts the timer. Call on every incoming DEADMAN_HOLD command.
func (w *DeadmanWatchdog) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.timer == nil {
		return
	}
	w.timer.Reset(w.timeout)
}

// Stop disables the watchdog — call on clean disconnect or SAFE_MODE entry.
func (w *DeadmanWatchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	if w.timer != nil {
		w.timer.Stop()
	}
	log.Printf("[DEADMAN] watchdog stopped (session=%s)", w.sessionID)
}

func (w *DeadmanWatchdog) fire() {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	sessionID := w.sessionID
	vehicleID := w.vehicleID
	w.stopped = true
	w.mu.Unlock()

	log.Printf("[DEADMAN] timeout — CRITICAL → SAFE_MODE (session=%s)", sessionID)
	w.sm.TransitionSystem(statemachine.StateSafeMode)
	w.publisher.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sessionID,
		VehicleID: vehicleID,
		Type:      safetyservice.EventDeadmanTimeout,
		Reason:    "dead-man switch timeout — operator released hold",
		Timestamp: time.Now(),
	})
}

// ACKTimeoutWatcher detects when the control server fails to ACK a command within budget.
// CommandReceived() starts the per-command timer. CommandACKed() cancels it.
// If the timer fires: CRITICAL → SAFE_MODE (ADR-009/010).
type ACKTimeoutWatcher struct {
	mu           sync.Mutex
	pendingTimer *time.Timer
	timeout      time.Duration
	sm           *statemachine.Machine
	publisher    Publisher
	sessionID    string
	vehicleID    string
}

func NewACKTimeoutWatcher(timeout time.Duration, sm *statemachine.Machine, publisher Publisher) *ACKTimeoutWatcher {
	return &ACKTimeoutWatcher{
		timeout:   timeout,
		sm:        sm,
		publisher: publisher,
	}
}

// CommandReceived starts a per-command ACK timer.
// Must be followed by CommandACKed() within Timeout, or SAFE_MODE is triggered.
func (w *ACKTimeoutWatcher) CommandReceived(sessionID, vehicleID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sessionID = sessionID
	w.vehicleID = vehicleID
	if w.pendingTimer != nil {
		w.pendingTimer.Stop()
	}
	w.pendingTimer = time.AfterFunc(w.timeout, w.fire)
}

// CommandACKed cancels the pending ACK timer. Call after sending ControlAck to the client.
func (w *ACKTimeoutWatcher) CommandACKed() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pendingTimer != nil {
		w.pendingTimer.Stop()
		w.pendingTimer = nil
	}
}

func (w *ACKTimeoutWatcher) fire() {
	w.mu.Lock()
	sessionID := w.sessionID
	vehicleID := w.vehicleID
	w.pendingTimer = nil
	w.mu.Unlock()

	log.Printf("[ACK] timeout exceeded %s — CRITICAL → SAFE_MODE (session=%s)", w.timeout, sessionID)
	w.sm.TransitionSystem(statemachine.StateSafeMode)
	w.publisher.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sessionID,
		VehicleID: vehicleID,
		Type:      safetyservice.EventACKTimeout,
		Reason:    "command ACK timeout — control loop budget exceeded",
		Timestamp: time.Now(),
	})
}
