package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"avoc/internal/safetyservice"
)

func main() {
	port := os.Getenv("SAFETY_PORT")
	if port == "" {
		port = "8082"
	}

	bus := safetyservice.NewBus()

	bus.Subscribe(func(event safetyservice.SafetyEvent) {
		log.Printf("[SAFETY] %s — session=%s vehicle=%s reason=%s",
			event.Type, event.SessionID, event.VehicleID, event.Reason)
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

	log.Printf("Safety Service starting on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Safety Service failed: %v", err)
	}
}
