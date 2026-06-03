package safety

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	"avoc/internal/safetyservice"
)

// HTTPPublisher publishes safety events to the Safety Event Bus service via HTTP (ADR-002).
type HTTPPublisher struct {
	baseURL string
	client  *http.Client
}

func NewHTTPPublisher(baseURL string) *HTTPPublisher {
	return &HTTPPublisher{baseURL: baseURL, client: &http.Client{}}
}

func (p *HTTPPublisher) TriggerEmergencyStop(sessionID, vehicleID, reason string) {
	body, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"vehicle_id": vehicleID,
		"reason":     reason,
	})
	p.post("/safety/emergency-stop", body)
}

func (p *HTTPPublisher) PublishEvent(event safetyservice.SafetyEvent) {
	body, _ := json.Marshal(event)
	p.post("/safety/event", body)
}

func (p *HTTPPublisher) post(path string, body []byte) {
	resp, err := p.client.Post(p.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[SAFETY] HTTPPublisher: failed to reach safety service at %s: %v", path, err)
		return
	}
	defer resp.Body.Close()
}
