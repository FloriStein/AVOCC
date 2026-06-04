package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"avoc/internal/controlserver/command"
	csafety "avoc/internal/controlserver/safety"
	"avoc/internal/controlserver/session"
	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/transport"
	"avoc/internal/recording"
	"avoc/internal/vehicleconnection"
	"avoc/pkg/ulid"
)

func main() {
	port := os.Getenv("CONTROL_PORT")
	if port == "" {
		port = "8080"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	safetyURL := envOr("SAFETY_SERVICE_URL", "http://safety-service:8082")
	sfuURL := envOr("SFU_SERVICE_URL", "http://webrtc-sfu:8084")
	authURL := envOr("AUTH_SERVICE_URL", "http://auth-service:8081")

	// --- Core components ---
	sm := statemachine.New()
	safetyPub := csafety.NewHTTPPublisher(safetyURL)
	sfuPub := session.NewHTTPSFUPublisher(sfuURL)
	sessionMgr := session.NewManager(sfuPub)
	deadman := csafety.NewDeadmanWatchdog(csafety.DefaultDeadmanTimeout, sm, safetyPub)
	ackWatcher := csafety.NewACKTimeoutWatcher(csafety.DefaultACKTimeout, sm, safetyPub)
	handoverMgr := session.NewHandoverManager(sm, sessionMgr, authURL)
	recorder := recording.NewMemoryRecorder()
	cmdEngine := command.NewEngine(sm, safetyPub, sessionMgr, deadman)

	// --- Handlers ---
	wsHandler := transport.NewWSHandler(secret, sm, sessionMgr, deadman, ackWatcher, cmdEngine)
	vehicleHandler := vehicleconnection.NewHandler(secret, sm, safetyPub)

	// --- Routes ---
	mux := http.NewServeMux()

	// Operator WebSocket (ADR-010)
	mux.HandleFunc("/ws", wsHandler.ServeWS)

	// Vehicle WebSocket (BE-06)
	mux.HandleFunc("/vehicle/ws", vehicleHandler.ServeWS)

	// Session lifecycle (GSA — ADR-015)
	mux.HandleFunc("POST /session/start", func(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("[SESSION] started: id=%s vehicle=%s operator=%s", sess.ID, sess.VehicleID, sess.OperatorID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"session_id": sess.ID})
	})

	mux.HandleFunc("POST /session/end", func(w http.ResponseWriter, _ *http.Request) {
		if sess, ok := sessionMgr.GetCurrentSession(); ok {
			recorder.EndSession(sess.ID)
		}
		sessionMgr.PushSFUEvent("SESSION_ENDED")
		sessionMgr.EndSession()
		sm.TransitionOperator(statemachine.OpNoOperator)
		w.WriteHeader(http.StatusNoContent)
	})

	// Operator Handover (BE-12)
	mux.HandleFunc("POST /handover/request", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("POST /handover/confirm", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("POST /handover/cancel", func(w http.ResponseWriter, _ *http.Request) {
		handoverMgr.CancelHandover()
		w.WriteHeader(http.StatusOK)
	})

	// MEDIA STATE update — frontend reports WebRTC state changes (ADR-009/011 Invariant 1)
	// MEDIA_FAILED → DEGRADED (never SAFE_MODE)
	mux.HandleFunc("POST /media/event", func(w http.ResponseWriter, r *http.Request) {
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
			sm.TransitionMedia(statemachine.MediaFailed) // → DEGRADED, never SAFE_MODE
		case "MEDIA_INIT":
			sm.TransitionMedia(statemachine.MediaInit)
		default:
			http.Error(w, "unknown media state", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	// Emergency Stop proxy — frontend calls this to avoid cross-origin issues (ADR-009)
	mux.HandleFunc("POST /emergency-stop", func(w http.ResponseWriter, r *http.Request) {
		sess, _ := sessionMgr.GetCurrentSession()
		safetyPub.TriggerEmergencyStop(sess.ID, sess.VehicleID, "operator emergency stop")
		sm.TransitionSystem(statemachine.StateSafeMode)
		if sess.ID != "" {
			sessionMgr.SaveCheckpoint("SAFE_MODE", "CONTROL_BLOCKED", "EMERGENCY_STOP")
			sessionMgr.PushSFUEvent("SESSION_SAFE_MODE")
			recorder.RecordSafetyEvent(sess.ID, ulid.Generate(), sess.VehicleID, sess.OperatorID, "EMERGENCY_STOP", "operator emergency stop")
		}
		w.WriteHeader(http.StatusAccepted)
	})

	// Recording inspection (ADR-005)
	mux.HandleFunc("GET /recording/", func(w http.ResponseWriter, r *http.Request) {
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
	})

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

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "control-server"})
	})

	log.Printf("Control Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Control Server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
