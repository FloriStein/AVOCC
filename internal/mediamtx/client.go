// Package mediamtx provides a client for the MediaMTX Management API (ADR-020).
// Used by the Control Server to kick all subscribers of a vehicle path on SAFE_MODE.
package mediamtx

import (
	"encoding/json"
	"fmt"
	"net/http"

	"avoc/pkg/logger"
)

var log = logger.New("mediamtx-client")

type Client struct {
	apiURL string
	http   *http.Client
}

func NewClient(apiURL string) *Client {
	return &Client{apiURL: apiURL, http: &http.Client{}}
}

type webrtcSession struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type webrtcSessionsResponse struct {
	Items []webrtcSession `json:"items"`
}

// KickVehicle terminates all active WebRTC sessions for the given vehicle path.
// Called by Control Server on SAFE_MODE to immediately stop all subscriber streams.
func (c *Client) KickVehicle(vehicleID string) {
	sessions, err := c.listSessions(vehicleID)
	if err != nil {
		log.Warn("mediamtx: could not list sessions", "vehicle_id", vehicleID, "error", err)
		return
	}
	for _, s := range sessions {
		if err := c.deleteSession(s.ID); err != nil {
			log.Warn("mediamtx: could not delete session", "session_id", s.ID, "error", err)
		} else {
			log.Info("mediamtx: subscriber kicked on SAFE_MODE", "session_id", s.ID, "vehicle_id", vehicleID)
		}
	}
}

func (c *Client) listSessions(vehicleID string) ([]webrtcSession, error) {
	url := fmt.Sprintf("%s/v3/webrtcsessions/list", c.apiURL)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result webrtcSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var filtered []webrtcSession
	for _, s := range result.Items {
		if s.Path == vehicleID {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (c *Client) deleteSession(sessionID string) error {
	url := fmt.Sprintf("%s/v3/webrtcsessions/kick/%s", c.apiURL, sessionID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("mediamtx kick returned %d", resp.StatusCode)
	}
	return nil
}
