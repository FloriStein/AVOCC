package session

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

// SFUPublisher pushes Session Events to the WebRTC SFU (ADR-015).
// The SFU consumes but never interprets — Dumb Media Router with State Subscription.
type SFUPublisher interface {
	PublishSessionEvent(eventType, sessionID, operatorID string)
}

// HTTPSFUPublisher sends session events to the WebRTC SFU via HTTP.
type HTTPSFUPublisher struct {
	baseURL string
	client  *http.Client
}

func NewHTTPSFUPublisher(baseURL string) *HTTPSFUPublisher {
	return &HTTPSFUPublisher{baseURL: baseURL, client: &http.Client{}}
}

func (p *HTTPSFUPublisher) PublishSessionEvent(eventType, sessionID, operatorID string) {
	body, _ := json.Marshal(map[string]string{
		"type":        eventType,
		"session_id":  sessionID,
		"operator_id": operatorID,
	})
	resp, err := p.client.Post(p.baseURL+"/session/event", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[SESSION] SFU event push failed (%s): %v", eventType, err)
		return
	}
	defer resp.Body.Close()
}
