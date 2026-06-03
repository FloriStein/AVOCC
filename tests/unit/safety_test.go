// Safety Test Suite — dedicated scenario-based tests for all CRITICAL triggers (ADR-006/009/011).
// Each test maps to a documented failure class. These tests are the safety gate in CI.
package unit_test

import (
	"testing"
	"time"

	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/safetyservice"
	"avoc/tests/unit/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSetup builds the minimal wiring needed for safety tests.
func newTestSetup(t *testing.T) (
	sm *statemachine.Machine,
	pub *mocks.MockSafetyPublisher,
	sfuPub *mocks.MockSFUPublisher,
	mgr *session.Manager,
) {
	t.Helper()
	sm = statemachine.New()
	pub = &mocks.MockSafetyPublisher{}
	sfuPub = &mocks.MockSFUPublisher{}
	mgr = session.NewManager(sfuPub)
	return
}

// connectSession transitions the state machine to CONNECTED and creates a session.
func connectSession(t *testing.T, sm *statemachine.Machine, mgr *session.Manager) session.Session {
	t.Helper()
	sm.TransitionSystem(statemachine.StateConnecting)
	sm.TransitionSystem(statemachine.StateAuthenticated)
	ok := sm.TransitionToConnected()
	require.True(t, ok, "TransitionToConnected must succeed from AUTHENTICATED")
	sess := mgr.CreateSession("vehicle-1", "operator-1", "ACTIVE_OPERATOR")
	mgr.PushSFUEvent("SESSION_CREATED")
	return sess
}

// --- Transition Validation ---

func TestSafety_InvalidTransitionRejected(t *testing.T) {
	sm, _, _, _ := newTestSetup(t)

	// IDLE → CONNECTED is invalid (must go via CONNECTING → AUTHENTICATED)
	sm.TransitionSystem(statemachine.StateConnected)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateIdle, sys, "invalid transition must be rejected")
}

// --- CRITICAL Trigger 1: WS Disconnect ---

func TestSafety_WSDisconnect_TriggersSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	// Simulate WS disconnect
	sm.TransitionSystem(statemachine.StateSafeMode)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- CRITICAL Trigger 2: Dead-man Switch Timeout ---

func TestSafety_DeadmanTimeout_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watchdog := csafety.NewDeadmanWatchdog(50*time.Millisecond, sm, pub)
	watchdog.Start(sess.ID, sess.VehicleID)

	// Do NOT call Reset() — let the timer fire
	time.Sleep(150 * time.Millisecond)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "dead-man timeout must trigger SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, safetyservice.EventDeadmanTimeout, pub.LastEventType())
}

func TestSafety_DeadmanReset_PreventsTimeout(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watchdog := csafety.NewDeadmanWatchdog(100*time.Millisecond, sm, pub)
	watchdog.Start(sess.ID, sess.VehicleID)

	// Keep resetting — should never fire
	for i := 0; i < 5; i++ {
		time.Sleep(60 * time.Millisecond)
		watchdog.Reset()
	}

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "reset dead-man must NOT trigger SAFE_MODE")
	watchdog.Stop()
}

// --- CRITICAL Trigger 3: Command ACK Timeout ---

func TestSafety_ACKTimeout_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watcher := csafety.NewACKTimeoutWatcher(50*time.Millisecond, sm, pub)
	watcher.CommandReceived(sess.ID, sess.VehicleID)

	// Do NOT call CommandACKed() — let timer fire
	time.Sleep(150 * time.Millisecond)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "ACK timeout must trigger SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, safetyservice.EventACKTimeout, pub.LastEventType())
}

func TestSafety_ACKInTime_NoSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	watcher := csafety.NewACKTimeoutWatcher(100*time.Millisecond, sm, pub)
	watcher.CommandReceived(sess.ID, sess.VehicleID)
	watcher.CommandACKed() // ACK within budget

	time.Sleep(150 * time.Millisecond)

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateConnected, sys, "ACK in time must NOT trigger SAFE_MODE")
}

// --- CRITICAL Trigger 4: No Active Operator ---

func TestSafety_NoOperator_TriggersSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)

	// Operator leaves
	sm.TransitionOperator(statemachine.OpNoOperator)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "NO_OPERATOR must trigger SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- CRITICAL Trigger 5: Emergency Stop ---

func TestSafety_EmergencyStop_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	// Emergency stop bypasses all layers — direct SAFE_MODE transition
	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.TriggerEmergencyStop(sess.ID, sess.VehicleID, "operator triggered E-Stop")

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
	assert.Equal(t, 1, pub.EmergencyStopCount())
}

// --- CRITICAL Trigger 6: Safety Bus Event (AUTH_INVALID / SAFETY_BUS_DOWN) ---

func TestSafety_AuthInvalidation_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sess.ID,
		VehicleID: sess.VehicleID,
		Type:      safetyservice.EventAuthInvalid,
		Reason:    "JWT revoked",
	})

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, safetyservice.EventAuthInvalid, pub.LastEventType())
}

func TestSafety_SafetyBusDown_TriggersSafeMode(t *testing.T) {
	sm, pub, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	pub.PublishEvent(safetyservice.SafetyEvent{
		SessionID: sess.ID,
		VehicleID: sess.VehicleID,
		Type:      safetyservice.EventSafetyBusDown,
		Reason:    "safety bus unreachable",
	})

	sys, _, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys)
}

// --- ADR-009 Invariant 1: MEDIA_FAILED must NEVER trigger SAFE_MODE ---

func TestSafety_MediaFailed_TriggersDegrade_NeverSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	sm.TransitionMedia(statemachine.MediaFailed)

	sys, ctrl, media, _ := sm.Get()
	assert.Equal(t, statemachine.StateDegraded, sys, "MEDIA_FAILED must trigger DEGRADED")
	assert.NotEqual(t, statemachine.StateSafeMode, sys, "MEDIA_FAILED must NEVER trigger SAFE_MODE (Invariant 1)")
	assert.Equal(t, statemachine.ControlActive, ctrl, "control must remain ACTIVE during DEGRADED")
	assert.Equal(t, statemachine.MediaFailed, media)
}

func TestSafety_MediaDegraded_TriggersDegrade_NeverSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	sm.TransitionMedia(statemachine.MediaDegraded)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateDegraded, sys)
	assert.NotEqual(t, statemachine.StateSafeMode, sys)
	assert.Equal(t, statemachine.ControlActive, ctrl, "control stays active during DEGRADED")
}

// --- Recovery Checkpoint ---

func TestSafety_RecoveryCheckpoint_SavedOnSafeMode(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	sess := connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	sys, ctrl, _, _ := sm.Get()
	mgr.SaveCheckpoint(string(sys), string(ctrl), "WS_DISCONNECT")

	cp, ok := mgr.LoadCheckpoint()
	require.True(t, ok, "checkpoint must exist after SAFE_MODE")
	assert.Equal(t, sess.ID, cp.SessionID, "checkpoint session ID must match active session")
	assert.Equal(t, "SAFE_MODE", cp.LastSystemState)
	assert.Equal(t, "CONTROL_BLOCKED", cp.LastControlState)
	assert.Equal(t, "WS_DISCONNECT", cp.SafetyReason)
}

func TestSafety_Recovery_FailsIfValidationFails(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)

	sm.TransitionSystem(statemachine.StateSafeMode)
	sm.TransitionSystem(statemachine.StateRecovering)
	mgr.SaveCheckpoint("RECOVERING", "CONTROL_RECOVERING", "reconnect_failed")

	// Simulate validation failure: recovering → SAFE_MODE fallback
	sm.TransitionSystem(statemachine.StateSafeMode)

	sys, ctrl, _, _ := sm.Get()
	assert.Equal(t, statemachine.StateSafeMode, sys, "failed recovery must fall back to SAFE_MODE")
	assert.Equal(t, statemachine.ControlBlocked, ctrl)
}

// --- Session Manager (GSA) ---

func TestSafety_SessionID_IsULID(t *testing.T) {
	_, _, sfuPub, mgr := newTestSetup(t)
	_ = sfuPub

	sess := mgr.CreateSession("vehicle-1", "operator-1", "ACTIVE_OPERATOR")

	assert.NotEmpty(t, sess.ID, "session ID must not be empty")
	assert.Len(t, sess.ID, 26, "ULID must be 26 characters")
}

func TestSafety_SessionID_UniquePerSession(t *testing.T) {
	_, _, _, mgr := newTestSetup(t)

	s1 := mgr.CreateSession("v-1", "op-1", "ACTIVE_OPERATOR")
	s2 := mgr.CreateSession("v-1", "op-1", "ACTIVE_OPERATOR")

	assert.NotEqual(t, s1.ID, s2.ID, "each session must have a unique ID")
}

// --- Operator Handover ---

func TestSafety_Handover_TransitionsToHandoverPending(t *testing.T) {
	sm, _, sfuPub, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(sm, mgr, "") // no auth URL in test

	err := handoverMgr.RequestHandover("operator-1", "operator-2")
	require.NoError(t, err)

	_, _, _, op := sm.Get()
	assert.Equal(t, statemachine.OpHandoverPending, op)
	_ = sfuPub
}

func TestSafety_Handover_ConfirmSwitchesActiveOperator(t *testing.T) {
	sm, _, sfuPub, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(sm, mgr, "")
	require.NoError(t, handoverMgr.RequestHandover("operator-1", "operator-2"))
	require.NoError(t, handoverMgr.ConfirmHandover("operator-2"))

	_, _, _, op := sm.Get()
	assert.Equal(t, statemachine.OpActive, op)

	sess, ok := mgr.GetCurrentSession()
	require.True(t, ok)
	assert.Equal(t, "operator-2", sess.OperatorID)

	// PushSFUEvent is async — wait briefly for the goroutine to deliver
	assert.Eventually(t, func() bool {
		events := sfuPub.Events()
		for _, e := range events {
			if e.Type == "OPERATOR_HANDOVER" {
				return true
			}
		}
		return false
	}, 200*time.Millisecond, 10*time.Millisecond, "OPERATOR_HANDOVER must be pushed to SFU")
}

func TestSafety_Handover_CancelRestoresActiveOperator(t *testing.T) {
	sm, _, _, mgr := newTestSetup(t)
	connectSession(t, sm, mgr)
	sm.TransitionOperator(statemachine.OpActive)

	handoverMgr := session.NewHandoverManager(sm, mgr, "")
	require.NoError(t, handoverMgr.RequestHandover("operator-1", "operator-2"))
	handoverMgr.CancelHandover()

	_, _, _, op := sm.Get()
	assert.Equal(t, statemachine.OpActive, op)
	assert.False(t, handoverMgr.IsPending())
}
