// Package transport implements the WebSocket Transport Layer (ADR-010).
// JWT auth in handshake, Channel Close on CRITICAL events, Heartbeat 30s,
// DeadmanWatchdog and ACKTimeoutWatcher integrated (BE-10).
package transport

import (
	"log"
	"net/http"
	"strings"
	"time"

	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

const heartbeatInterval = 30 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

type WSHandler struct {
	jwtSecret  []byte
	sm         *statemachine.Machine
	sessionMgr *session.Manager
	deadman    *csafety.DeadmanWatchdog
	ackWatcher *csafety.ACKTimeoutWatcher
}

func NewWSHandler(
	jwtSecret string,
	sm *statemachine.Machine,
	sessionMgr *session.Manager,
	deadman *csafety.DeadmanWatchdog,
	ackWatcher *csafety.ACKTimeoutWatcher,
) *WSHandler {
	return &WSHandler{
		jwtSecret:  []byte(jwtSecret),
		sm:         sm,
		sessionMgr: sessionMgr,
		deadman:    deadman,
		ackWatcher: ackWatcher,
	}
}

// ServeWS upgrades the connection and validates JWT in the handshake (ADR-004).
// On success transitions CONNECTING → AUTHENTICATED (ADR-011).
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractToken(r)
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := h.validateJWT(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Recovery path: SAFE_MODE → RECOVERING → AUTHENTICATED (ADR-009/011)
	// Normal path:   IDLE → CONNECTING → AUTHENTICATED
	if current, _, _, _ := h.sm.Get(); current == statemachine.StateSafeMode {
		h.sm.TransitionSystem(statemachine.StateRecovering)
	} else {
		h.sm.TransitionSystem(statemachine.StateConnecting)
	}
	h.sm.TransitionSystem(statemachine.StateAuthenticated)

	log.Printf("WebSocket connected: subject=%s role=%s", claims.Subject, claims.Role)

	go h.heartbeat(conn)
	h.readLoop(conn, claims)
}

func (h *WSHandler) heartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

func (h *WSHandler) readLoop(conn *websocket.Conn, claims *Claims) {
	defer func() {
		h.deadman.Stop()
		sysState, _, _, _ := h.sm.Get()
		if sysState != statemachine.StateSafeMode {
			// WS disconnect → CRITICAL → SAFE_MODE + Channel Close (ADR-009/010)
			h.sm.TransitionSystem(statemachine.StateSafeMode)
			log.Printf("WebSocket disconnected: WS_DISCONNECT → SAFE_MODE (subject=%s)", claims.Subject)
		}
		// Save recovery checkpoint on SAFE_MODE entry
		sys, ctrl, _, _ := h.sm.Get()
		if sess, ok := h.sessionMgr.GetCurrentSession(); ok {
			h.sessionMgr.SaveCheckpoint(string(sys), string(ctrl), "WS_DISCONNECT")
			h.sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
			log.Printf("[SESSION] checkpoint saved (session=%s)", sess.ID)
		}
	}()

	conn.SetPongHandler(func(_ string) error {
		conn.SetReadDeadline(time.Now().Add(heartbeatInterval * 2))
		return nil
	})

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		sysState, ctrlState, _, _ := h.sm.Get()
		if sysState == statemachine.StateSafeMode || ctrlState == statemachine.ControlBlocked {
			// Commands dropped silently in SAFE_MODE (ADR-011)
			continue
		}

		sess, hasSession := h.sessionMgr.GetCurrentSession()

		// Reset dead-man watchdog on every command (BE-10).
		// TODO BE-04: only reset on COMMAND_TYPE_DEADMAN_HOLD after protobuf parsing.
		h.deadman.Reset()

		if hasSession {
			// Start ACK timeout window (ADR-010).
			h.ackWatcher.CommandReceived(sess.ID, sess.VehicleID)
		}

		log.Printf("Control command received (%d bytes)", len(msg))
		// TODO BE-04: route to Command Engine (parse protobuf ControlCommand)

		// Send immediate ACK — cancels the ACK timeout window.
		// TODO BE-04: send proper ControlAck protobuf response.
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"ack":true}`)); err != nil {
			h.ackWatcher.CommandACKed()
			return
		}
		h.ackWatcher.CommandACKed()
	}
}

func (h *WSHandler) validateJWT(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return h.jwtSecret, nil
	})
	return claims, err
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
