package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"avoc/internal/webrtcsfu"
)

func main() {
	port := os.Getenv("SFU_PORT")
	if port == "" {
		port = "8084"
	}

	sfu := webrtcsfu.New()
	mux := http.NewServeMux()

	// Session Event Consumer — receives SESSION_* events from Control Server (ADR-015).
	// The SFU is a Dumb Media Router: it consumes but never interprets safety state.
	mux.HandleFunc("POST /session/event", func(w http.ResponseWriter, r *http.Request) {
		var event webrtcsfu.SessionEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		sfu.HandleSessionEvent(event)
		w.WriteHeader(http.StatusAccepted)
	})

	// Vehicle WebRTC Offer — vehicle sends SDP offer, SFU returns SDP answer.
	mux.HandleFunc("POST /offer/{sessionId}/{peerId}", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionId")
		peerID := r.PathValue("peerId")

		var req struct {
			SDP string `json:"sdp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		answer, err := sfu.CreateVehicleOffer(sessionID, peerID, req.SDP)
		if err != nil {
			log.Printf("[SFU] offer error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"sdp": answer})
	})

	// Operator WebRTC subscription — browser sends SDP offer, SFU returns SDP answer.
	// Tracks from the vehicle are forwarded to this operator connection (ADR-014).
	mux.HandleFunc("POST /subscribe/{sessionId}/{operatorId}", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionId")
		operatorID := r.PathValue("operatorId")

		var req struct {
			SDP string `json:"sdp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		answer, err := sfu.SubscribeOperator(sessionID, operatorID, req.SDP)
		if err != nil {
			log.Printf("[SFU] subscribe error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"sdp": answer})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "webrtc-sfu"})
	})

	log.Printf("WebRTC SFU starting on :%s (ADR-014)", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("WebRTC SFU failed: %v", err)
	}
}
