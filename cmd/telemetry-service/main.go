package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("TELEMETRY_PORT")
	if port == "" {
		port = "8083"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "telemetry-service"})
	})

	log.Printf("Telemetry Service starting on :%s (Sprint 4 implementation pending)", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Telemetry Service failed: %v", err)
	}
}
