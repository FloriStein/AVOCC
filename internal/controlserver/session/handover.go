package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"avoc/internal/controlserver/statemachine"
	"avoc/pkg/logger"
)

var svcLog = logger.New("control-server")

// HandoverManager coordinates operator handover (ADR-011/015).
// Rules: max 1 ACTIVE_OPERATOR, both sides must confirm, current operator retains
// control during HANDOVER_PENDING, SFU notified immediately on completion.
type HandoverManager struct {
	mu         sync.Mutex
	sm         *statemachine.Machine
	sessions   *Manager
	authURL    string
	httpClient *http.Client
	pending    *pendingHandover
}

type pendingHandover struct {
	fromOperatorID string
	toOperatorID   string
}

func NewHandoverManager(sm *statemachine.Machine, sessions *Manager, authURL string) *HandoverManager {
	return &HandoverManager{
		sm:         sm,
		sessions:   sessions,
		authURL:    authURL,
		httpClient: &http.Client{},
	}
}

// RequestHandover initiates a handover — transitions OPERATOR STATE to HANDOVER_PENDING.
// The current operator retains control until ConfirmHandover is called (ADR-011).
func (h *HandoverManager) RequestHandover(fromOperatorID, toOperatorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, _, _, opState := h.sm.Get()
	if opState != statemachine.OpActive {
		return fmt.Errorf("handover requires ACTIVE_OPERATOR state, got %s", opState)
	}

	h.pending = &pendingHandover{
		fromOperatorID: fromOperatorID,
		toOperatorID:   toOperatorID,
	}

	h.sm.TransitionOperator(statemachine.OpHandoverPending)
	svcLog.Info("handover requested",
		"from_operator", fromOperatorID, "to_operator", toOperatorID)
	return nil
}

// ConfirmHandover completes the handover — target becomes ACTIVE_OPERATOR.
// Issues a new ACTIVE_OPERATOR token via Auth Service and notifies the SFU (ADR-015).
func (h *HandoverManager) ConfirmHandover(confirmingOperatorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pending == nil {
		return fmt.Errorf("no handover pending")
	}
	if confirmingOperatorID != h.pending.toOperatorID {
		return fmt.Errorf("confirming operator %s is not the handover target %s",
			confirmingOperatorID, h.pending.toOperatorID)
	}

	if err := h.issueHandoverToken(h.pending.toOperatorID); err != nil {
		return fmt.Errorf("handover token issuance failed: %w", err)
	}

	h.sessions.UpdateOperator(h.pending.toOperatorID, string(statemachine.OpActive))
	h.sm.TransitionOperator(statemachine.OpActive)
	h.sessions.PushSFUEvent("OPERATOR_HANDOVER")

	svcLog.Event(logger.EventOperatorHandover, "handover confirmed",
		"new_active_operator", h.pending.toOperatorID)
	h.pending = nil
	return nil
}

// CancelHandover aborts the handover — current operator retains control.
func (h *HandoverManager) CancelHandover() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pending == nil {
		return
	}
	svcLog.Info("handover cancelled",
		"from_operator", h.pending.fromOperatorID, "to_operator", h.pending.toOperatorID)
	h.pending = nil
	h.sm.TransitionOperator(statemachine.OpActive)
}

// IsPending returns true if a handover is currently in progress.
func (h *HandoverManager) IsPending() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pending != nil
}

func (h *HandoverManager) issueHandoverToken(targetOperatorID string) error {
	if h.authURL == "" {
		return nil // no auth service configured (e.g. in tests)
	}
	body, _ := json.Marshal(map[string]string{"target_id": targetOperatorID})
	resp, err := h.httpClient.Post(h.authURL+"/auth/handover/token", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("auth service returned %d", resp.StatusCode)
	}
	return nil
}
