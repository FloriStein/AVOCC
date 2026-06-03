package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("SFU_PORT")
	if port == "" {
		port = "8084"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "webrtc-sfu"})
	})

	log.Printf("WebRTC SFU starting on :%s (Sprint 4 implementation pending — ADR-014)", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("WebRTC SFU failed: %v", err)
	}
}
