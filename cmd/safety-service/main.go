package main

import (
	"encoding/json"
	"net/http"
	"os"

	"avoc/internal/safetyservice"
	"avoc/pkg/logger"
)

var log = logger.New("safety-service")

func main() {
	port := os.Getenv("SAFETY_PORT")
	if port == "" {
		port = "8082"
	}

	bus := safetyservice.NewBus()

	bus.Subscribe(func(event safetyservice.SafetyEvent) {
		log.Event(string(event.Type), "safety event received",
			"session_id", event.SessionID,
			"vehicle_id", event.VehicleID,
			"reason", event.Reason)
	})

	mux := http.NewServeMux()

	mux.HandleFunc("POST /safety/event", func(w http.ResponseWriter, r *http.Request) {
		var event safetyservice.SafetyEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		bus.PublishSafetyEvent(event)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /safety/emergency-stop", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SessionID string `json:"session_id"`
			VehicleID string `json:"vehicle_id"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		bus.TriggerEmergencyStop(req.SessionID, req.VehicleID, req.Reason)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("GET /safety/state", func(w http.ResponseWriter, r *http.Request) {
		state := bus.GetSafetyState()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "safety-service"})
	})

	log.Info("Safety Service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Safety Service failed", "error", err)
	}
}
