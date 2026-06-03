package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"avoc/internal/controlserver/statemachine"
	"avoc/internal/controlserver/transport"
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

	sm := statemachine.New()
	wsHandler := transport.NewWSHandler(secret, sm)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler.ServeWS)

	mux.HandleFunc("GET /state", func(w http.ResponseWriter, _ *http.Request) {
		sys, ctrl, media, op := sm.Get()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"system":   string(sys),
			"control":  string(ctrl),
			"media":    string(media),
			"operator": string(op),
		})
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
