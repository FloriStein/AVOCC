// Package transport implements the WebSocket Transport Layer (ADR-010).
// JWT auth in handshake, Channel Close on CRITICAL events, Heartbeat 30s.
package transport

import (
	"log"
	"net/http"
	"strings"
	"time"

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
	jwtSecret []byte
	sm        *statemachine.Machine
}

func NewWSHandler(jwtSecret string, sm *statemachine.Machine) *WSHandler {
	return &WSHandler{
		jwtSecret: []byte(jwtSecret),
		sm:        sm,
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

	h.sm.TransitionSystem(statemachine.StateConnecting)
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
		sysState, _, _, _ := h.sm.Get()
		if sysState != statemachine.StateSafeMode {
			// WS disconnect → CRITICAL → SAFE_MODE + Channel Close (ADR-009/010)
			h.sm.TransitionSystem(statemachine.StateSafeMode)
			log.Printf("WebSocket disconnected: WS_DISCONNECT → SAFE_MODE (subject=%s)", claims.Subject)
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
			// Commands are dropped silently in SAFE_MODE (ADR-011)
			continue
		}

		log.Printf("Control command received (%d bytes)", len(msg))
		// TODO BE-04: route to Command Engine
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
	// Check Authorization header first, then query param for WS handshake
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
