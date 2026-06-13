package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/transport"
	"avoc/internal/mediamtx"
	"avoc/internal/recording"
	"avoc/internal/vehicleconnection"
	"avoc/internal/vehicleregistry"
	"avoc/pkg/audit"
	pkgdb "avoc/pkg/db"
	"avoc/pkg/logger"
	"avoc/pkg/ulid"
)

var log = logger.New("control-server")

// feLog logs frontend events received via POST /log (service label: "frontend")
var feLog = logger.New("frontend")

func main() {
	port := envOr("CONTROL_PORT", "8080")
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	safetyURL := envOr("SAFETY_SERVICE_URL", "http://safety-service:8082")
	sfuURL := envOr("SFU_SERVICE_URL", "http://webrtc-sfu:8084")
	authURL := envOr("AUTH_SERVICE_URL", "http://auth-service:8081")
	whipStreamKey := os.Getenv("WHIP_STREAM_KEY")
	mediamtxAPIURL := envOr("MEDIAMTX_API_URL", "http://mediamtx:9997")
	turnExternalIP := os.Getenv("TURN_EXTERNAL_IP")
	turnPort := envOr("TURN_PORT", "3478")
	turnUser := os.Getenv("TURN_USER")
	turnPassword := os.Getenv("TURN_PASSWORD")
	mtxClient := mediamtx.NewClient(mediamtxAPIURL)

	// --- PostgreSQL (ADR-023) ---
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	db, err := pkgdb.Open(databaseURL)
	if err != nil {
		log.Fatal("failed to open database", "error", err)
	}
	defer db.Close()

	// --- Audit Writer (LOG-10/11 — ADR-018/023) ---
	var auditWriter audit.AuditWriter
	pgWriter, err := audit.NewPostgresAuditWriter(db)
	if err != nil {
		log.Warn("audit store unavailable — using NoopWriter", "error", err)
		auditWriter = audit.NewNoopWriter()
	} else {
		auditWriter = pgWriter
		defer pgWriter.Close()
		log.Info("audit store ready (PostgreSQL)")
	}

	// --- Core components ---
	sm := statemachine.New()
	safetyPub := csafety.NewHTTPPublisher(safetyURL)
	sfuPub := session.NewHTTPSFUPublisher(sfuURL)
	sessionMgr := session.NewManager(sfuPub)
	deadman := csafety.NewDeadmanWatchdog(csafety.DefaultDeadmanTimeout, sm, safetyPub).
		WithAuditWriter(auditWriter)
	ackWatcher := csafety.NewACKTimeoutWatcher(csafety.DefaultACKTimeout, sm, safetyPub).
		WithAuditWriter(auditWriter)
	handoverMgr := session.NewHandoverManager(sm, sessionMgr, authURL)
	recorder := recording.NewMemoryRecorder()
	vehicleRegistry := vehicleconnection.NewRegistry()
	vehicleAckStore := vehicleconnection.NewAckStore()

	// --- Vehicle Registry (ADR-022/023) ---
	var vehicleStore vehicleregistry.VehicleStore
	vs, vsErr := vehicleregistry.NewPostgresVehicleStore(db, vehicleRegistry)
	if vsErr != nil {
		log.Warn("vehicle registry unavailable", "error", vsErr)
		vehicleStore = vehicleregistry.NoopVehicleStore{}
	} else {
		vehicleStore = vs
		if seedErr := vs.SeedDefault(); seedErr != nil {
			log.Warn("vehicle registry seed failed", "error", seedErr)
		} else {
			log.Info("vehicle registry ready, vehicle-001 seeded")
		}
	}

	cmdEngine := command.NewEngine(sm, safetyPub, sessionMgr, deadman).
		WithAuditWriter(auditWriter).
		WithVehicleForwarder(vehicleRegistry)

	// --- Handlers ---
	wsHandler := transport.NewWSHandler(secret, sm, sessionMgr, deadman, ackWatcher, cmdEngine).
		WithAuditWriter(auditWriter)
	vehicleHandler := vehicleconnection.NewHandler(secret, sm, safetyPub, vehicleRegistry, vehicleAckStore)

	// --- Routes ---
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", wsHandler.ServeWS)
	mux.HandleFunc("/vehicle/ws", vehicleHandler.ServeWS)

	auth := requireJWT([]byte(secret))

	// Session lifecycle (GSA — ADR-015)
	mux.HandleFunc("POST /session/start", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			VehicleID    string `json:"vehicle_id"`
			OperatorID   string `json:"operator_id"`
			OperatorRole string `json:"operator_role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if !sm.TransitionToConnected() {
			http.Error(w, "system must be in AUTHENTICATED state", http.StatusConflict)
			return
		}
		sess := sessionMgr.CreateSession(req.VehicleID, req.OperatorID, req.OperatorRole)
		sm.TransitionOperator(statemachine.OpActive)
		deadman.Start(sess.ID, sess.VehicleID)
		sessionMgr.PushSFUEvent("SESSION_CREATED")
		recorder.StartSession(sess.ID, sess.VehicleID, sess.OperatorID)
		sys, ctrl, _, _ := sm.Get()
		recorder.RecordStateSnapshot(sess.ID, ulid.Generate(), sess.VehicleID, sess.OperatorID, string(sys), string(ctrl))
		log.Event(logger.EventSessionStarted, "session started",
			"session_id", sess.ID, "vehicle_id", sess.VehicleID, "operator_id", sess.OperatorID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"session_id": sess.ID})
	}))

	mux.HandleFunc("POST /session/end", auth(func(w http.ResponseWriter, _ *http.Request) {
		if sess, ok := sessionMgr.GetCurrentSession(); ok {
			recorder.EndSession(sess.ID)
			log.Event(logger.EventSessionEnded, "session ended", "session_id", sess.ID)
		}
		sessionMgr.PushSFUEvent("SESSION_ENDED")
		sessionMgr.EndSession()
		sm.TransitionOperator(statemachine.OpNoOperator)
		w.WriteHeader(http.StatusNoContent)
	}))

	// Operator Handover (BE-12)
	mux.HandleFunc("POST /handover/request", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			FromOperatorID string `json:"from_operator_id"`
			ToOperatorID   string `json:"to_operator_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if err := handoverMgr.RequestHandover(req.FromOperatorID, req.ToOperatorID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	mux.HandleFunc("POST /handover/confirm", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OperatorID string `json:"operator_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if err := handoverMgr.ConfirmHandover(req.OperatorID); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("POST /handover/cancel", auth(func(w http.ResponseWriter, _ *http.Request) {
		handoverMgr.CancelHandover()
		w.WriteHeader(http.StatusOK)
	}))

	// MEDIA STATE update (ADR-009/011 Invariant 1)
	mux.HandleFunc("POST /media/event", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			State string `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		switch req.State {
		case "MEDIA_NEGOTIATING":
			sm.TransitionMedia(statemachine.MediaNegotiating)
		case "MEDIA_CONNECTED":
			sm.TransitionMedia(statemachine.MediaConnected)
		case "MEDIA_DEGRADED":
			sm.TransitionMedia(statemachine.MediaDegraded)
		case "MEDIA_FAILED":
			sm.TransitionMedia(statemachine.MediaFailed)
		case "MEDIA_INIT":
			sm.TransitionMedia(statemachine.MediaInit)
		default:
			http.Error(w, "unknown media state", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	// Emergency Stop proxy (ADR-009)
	mux.HandleFunc("POST /emergency-stop", auth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := sessionMgr.GetCurrentSession()
		safetyPub.TriggerEmergencyStop(sess.ID, sess.VehicleID, "operator emergency stop")
		sm.TransitionSystem(statemachine.StateSafeMode)
		if sess.ID != "" {
			sessionMgr.SaveCheckpoint("SAFE_MODE", "CONTROL_BLOCKED", "EMERGENCY_STOP")
			sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
			recorder.RecordSafetyEvent(sess.ID, ulid.Generate(), sess.VehicleID, sess.OperatorID, "EMERGENCY_STOP", "operator emergency stop")
			// ADR-020: Control Server kicks MediaMTX subscribers directly on SAFE_MODE
			go mtxClient.KickVehicle(sess.VehicleID)
		}
		w.WriteHeader(http.StatusAccepted)
	}))

	// LOG-07: Frontend log ingestion — Browser → POST /log → slog (service="frontend") → Loki
	mux.HandleFunc("POST /log", func(w http.ResponseWriter, r *http.Request) {
		var entry struct {
			Level      string         `json:"level"`
			EventType  string         `json:"event_type"`
			SessionID  string         `json:"session_id"`
			VehicleID  string         `json:"vehicle_id"`
			OperatorID string         `json:"operator_id"`
			EventID    string         `json:"event_id"`
			Message    string         `json:"msg"`
			Data       map[string]any `json:"data,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if entry.Message == "" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		feLog.Event(entry.EventType, entry.Message,
			"session_id", entry.SessionID,
			"vehicle_id", entry.VehicleID,
			"operator_id", entry.OperatorID,
			"event_id", entry.EventID,
		)
		w.WriteHeader(http.StatusAccepted)
	})

	// LOG-11: Audit events query endpoint
	mux.HandleFunc("GET /audit/events", auth(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "session_id required", http.StatusBadRequest)
			return
		}
		events, err := auditWriter.QueryBySession(sessionID)
		if err != nil {
			log.Error("audit query failed", "error", err, "session_id", sessionID)
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}))

	// Recording inspection (ADR-005)
	mux.HandleFunc("GET /recording/", auth(func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimPrefix(r.URL.Path, "/recording/")
		if sessionID == "" {
			http.Error(w, "session_id required", http.StatusBadRequest)
			return
		}
		entries := recorder.GetEntries(sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"session_id": sessionID,
			"count":      len(entries),
			"entries":    entries,
		})
	}))

	// MediaMTX Auth-Hook (ADR-020) — validiert WHIP publish + WHEP read
	// MediaMTX ruft diesen Endpoint für jeden eingehenden WHIP/WHEP-Request auf.
	mux.HandleFunc("POST /internal/media/auth", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		log.Info("media auth: raw request", "body", string(bodyBytes))
		var req struct {
			Action string `json:"action"`
			Path   string `json:"path"`
			Token  string `json:"token"`  // Bearer Token (WHIP/WHEP via Authorization header)
		}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "publish":
			// WHIP: Fahrzeug-Client (Larix) authentifiziert sich mit Stream Key
			if whipStreamKey == "" || req.Token != whipStreamKey {
				log.Warn("media auth: WHIP publish rejected", "path", req.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			log.Info("media auth: WHIP publish allowed", "path", req.Path)
		case "read":
			// WHEP: Operator-Browser authentifiziert sich mit JWT
			// Prüfung: Token nicht leer + aktive Session vorhanden
			if req.Token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, hasSession := sessionMgr.GetCurrentSession()
			if !hasSession {
				log.Warn("media auth: WHEP read rejected — no active session", "path", req.Path)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			log.Info("media auth: WHEP read allowed", "path", req.Path)
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// VEH-07 (ADR-021): Latest VehicleCommandAck per vehicle — polled by frontend
	mux.HandleFunc("GET /vehicle/ack/latest/", func(w http.ResponseWriter, r *http.Request) {
		vehicleID := strings.TrimPrefix(r.URL.Path, "/vehicle/ack/latest/")
		if vehicleID == "" {
			http.Error(w, "vehicle_id required", http.StatusBadRequest)
			return
		}
		ack, ok := vehicleAckStore.Latest(vehicleID)
		if !ok {
			http.Error(w, "no ack for vehicle", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"vehicle_id":        vehicleID,
			"command_event_id":  ack.CommandEventId,
			"received":          ack.Received,
			"received_at_ms":    ack.ReceivedAtMs,
			"vehicle_connected": vehicleRegistry.Connected(vehicleID),
		})
	})

	// DEV: Stream key for browser-based WHIP sender (StreamSenderPanel).
	// The stream key is a shared secret known to the publisher — exposing it here
	// only saves the developer from manually copying it from .env.
	mux.HandleFunc("GET /dev/whip-key", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"streamKey": whipStreamKey})
	})

	// Vehicle Registry (ADR-022)
	mux.HandleFunc("GET /vehicles", func(w http.ResponseWriter, _ *http.Request) {
		vehicles, err := vehicleStore.List()
		if err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		type vehicleJSON struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
			Online      bool   `json:"online"`
		}
		result := make([]vehicleJSON, len(vehicles))
		for i, v := range vehicles {
			result[i] = vehicleJSON{ID: v.ID, DisplayName: v.DisplayName, Description: v.Description, Online: v.Online}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("POST /vehicles", auth(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.ID == "" || req.DisplayName == "" {
			http.Error(w, "id and display_name required", http.StatusBadRequest)
			return
		}
		exists, err := vehicleStore.Exists(req.ID)
		if err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "vehicle already exists", http.StatusConflict)
			return
		}
		if err := vehicleStore.Add(req.ID, req.DisplayName, req.Description); err != nil {
			http.Error(w, "add failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))

	mux.HandleFunc("DELETE /vehicles/{id}", auth(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		if sess, ok := sessionMgr.GetCurrentSession(); ok && sess.VehicleID == id {
			http.Error(w, "vehicle is currently in active session", http.StatusConflict)
			return
		}
		if err := vehicleStore.Delete(id); err != nil {
			if errors.Is(err, vehicleregistry.ErrNotFound) {
				http.Error(w, "vehicle not found", http.StatusNotFound)
				return
			}
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// State + Health
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, _ *http.Request) {
		sys, ctrl, media, op := sm.Get()
		resp := map[string]string{
			"system":   string(sys),
			"control":  string(ctrl),
			"media":    string(media),
			"operator": string(op),
		}
		if sess, ok := sessionMgr.GetCurrentSession(); ok {
			resp["session_id"] = sess.ID
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// ICE server config for WebRTC clients (WEBRTC-06 — Sprint 10).
	// Returns STUN + TURN (UDP + TCP) servers so the browser can gather its own
	// ICE candidates. No auth required — TURN credentials are per-design visible
	// to anyone who loads the page (they're transmitted in WebRTC signalling anyway).
	mux.HandleFunc("GET /ice-config", func(w http.ResponseWriter, _ *http.Request) {
		type iceServer struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		}
		host := turnExternalIP
		servers := []iceServer{
			{URLs: []string{"stun:" + host + ":" + turnPort}},
			{URLs: []string{"turn:" + host + ":" + turnPort}, Username: turnUser, Credential: turnPassword},
			{URLs: []string{"turn:" + host + ":" + turnPort + "?transport=tcp"}, Username: turnUser, Credential: turnPassword},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"iceServers": servers})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "control-server"})
	})

	log.Info("Control Server starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Control Server failed", "error", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireJWT returns middleware that enforces a valid operator JWT in the Authorization header.
// Returns 401 when the token is missing, malformed, or signed with a different secret.
func requireJWT(secret []byte) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			token, err := jwt.Parse(strings.TrimPrefix(authHeader, "Bearer "),
				func(t *jwt.Token) (any, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
					}
					return secret, nil
				})
			if err != nil || !token.Valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
}
