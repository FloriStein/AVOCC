// Package transport implements the WebSocket Transport Layer (ADR-010).
// JWT auth in handshake, Channel Close on CRITICAL events, Heartbeat 30s,
// DeadmanWatchdog and ACKTimeoutWatcher integrated (BE-10).
package transport

import (
	"net/http"
	"strings"
	"time"

	commonv1 "avoc/gen/go/common/v1"
	controlv1 "avoc/gen/go/control/v1"
	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/pkg/audit"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var svcLog = logger.New("control-server")

const heartbeatInterval = 30 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

type WSHandler struct {
	jwtSecret   []byte
	sm          *statemachine.Machine
	sessionMgr  *session.Manager
	deadman     *csafety.DeadmanWatchdog
	ackWatcher  *csafety.ACKTimeoutWatcher
	engine      *command.Engine
	auditWriter audit.AuditWriter
}

func NewWSHandler(
	jwtSecret string,
	sm *statemachine.Machine,
	sessionMgr *session.Manager,
	deadman *csafety.DeadmanWatchdog,
	ackWatcher *csafety.ACKTimeoutWatcher,
	engine *command.Engine,
) *WSHandler {
	return &WSHandler{
		jwtSecret:  []byte(jwtSecret),
		sm:         sm,
		sessionMgr: sessionMgr,
		deadman:    deadman,
		ackWatcher: ackWatcher,
		engine:     engine,
	}
}

// WithAuditWriter sets the audit writer for WS_DISCONNECT persistence (ADR-018).
func (h *WSHandler) WithAuditWriter(aw audit.AuditWriter) *WSHandler {
	h.auditWriter = aw
	return h
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
		svcLog.Error("WebSocket upgrade failed", "error", err)
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

	svcLog.Info("WebSocket connected", "subject", claims.Subject, "role", claims.Role)

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
			svcLog.Event(logger.EventWsDisconnect,
				"WebSocket disconnected → SAFE_MODE",
				"subject", claims.Subject)

			// Write to audit store BEFORE state transition (ADR-018)
			if h.auditWriter != nil {
				sys, ctrl, _, _ := h.sm.Get()
				if sess, ok := h.sessionMgr.GetCurrentSession(); ok {
					if err := h.auditWriter.WriteSync(audit.SafetyAuditEvent{
						EventID:     ulid.Generate(),
						SessionID:   sess.ID,
						VehicleID:   sess.VehicleID,
						OperatorID:  sess.OperatorID,
						EventType:   logger.EventWsDisconnect,
						Reason:      "operator WebSocket disconnected",
						SystemState: string(sys),
						CtrlState:   string(ctrl),
						Timestamp:   time.Now(),
					}); err != nil {
						svcLog.Error("audit write failed — proceeding to SAFE_MODE", "error", err)
					}
				}
			}

			h.sm.TransitionSystem(statemachine.StateSafeMode)
		}
		// Save recovery checkpoint on SAFE_MODE entry
		sys, ctrl, _, _ := h.sm.Get()
		if sess, ok := h.sessionMgr.GetCurrentSession(); ok {
			h.sessionMgr.SaveCheckpoint(string(sys), string(ctrl), "WS_DISCONNECT")
			h.sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
			svcLog.Event(logger.EventSafeModeEntered,
				"recovery checkpoint saved",
				"session_id", sess.ID)
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

		if hasSession {
			h.ackWatcher.CommandReceived(sess.ID, sess.VehicleID)
		}

		var ackBytes []byte
		if hasSession {
			var err error
			ackBytes, err = h.engine.Handle(msg, sess)
			if err != nil {
				ackBytes, _ = proto.Marshal(&controlv1.ControlAck{
					Header:   &commonv1.CorrelationHeader{Timestamp: time.Now().UnixMilli()},
					Success:  false,
					ErrorMsg: "command engine error",
				})
			}
		} else {
			ackBytes, _ = proto.Marshal(&controlv1.ControlAck{
				Header:   &commonv1.CorrelationHeader{Timestamp: time.Now().UnixMilli()},
				Success:  false,
				ErrorMsg: "no active session",
			})
		}

		if err := conn.WriteMessage(websocket.BinaryMessage, ackBytes); err != nil {
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
