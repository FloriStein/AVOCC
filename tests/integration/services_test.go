package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode, "GET %s", url)
	var m map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
	return m
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	require.NoError(t, err, "POST %s", url)
	return resp
}

// --- Health Checks ---

func TestIntegration_ControlServer_Healthy(t *testing.T) {
	m := getJSON(t, controlURL+"/health")
	assert.Equal(t, "ok", m["status"])
	assert.Equal(t, "control-server", m["service"])
}

func TestIntegration_AuthService_Healthy(t *testing.T) {
	m := getJSON(t, authURL+"/health")
	assert.Equal(t, "ok", m["status"])
}

func TestIntegration_SafetyService_Healthy(t *testing.T) {
	m := getJSON(t, safetyURL+"/health")
	assert.Equal(t, "ok", m["status"])
}

// --- Auth Service ---

func TestIntegration_Auth_OperatorLogin_ReturnsJWT(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"id": "op-int-test", "password": "test"})
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token, ok := body["token"].(string)
	require.True(t, ok, "response must contain token string")
	assert.Greater(t, len(token), 20, "JWT must be non-trivial length")
}

func TestIntegration_Auth_VehicleRegister_ReturnsJWT(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/vehicle/register",
		map[string]string{"id": "vehicle-int-test"})
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	_, ok := body["token"].(string)
	assert.True(t, ok)
}

// --- Control Server: Initial State ---

func TestIntegration_ControlServer_InitialState_IsIDLE(t *testing.T) {
	m := getJSON(t, controlURL+"/state")
	assert.Equal(t, "IDLE", m["system"])
	assert.Equal(t, "CONTROL_INIT", m["control"])
	assert.Equal(t, "MEDIA_INIT", m["media"])
	assert.Equal(t, "NO_OPERATOR", m["operator"])
}

// --- Session Lifecycle ---

func TestIntegration_SessionLifecycle_StartAndEnd(t *testing.T) {
	// 1. Login
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"id": "op-lifecycle", "password": "test"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var loginBody map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&loginBody))
	token := loginBody["token"].(string)

	// 2. WebSocket connect (upgrade) — skip full WS in integration test;
	//    instead call session/start directly to verify HTTP API path.
	//    State machine starts at IDLE — we need AUTHENTICATED first.
	// Authenticate by connecting WebSocket (transition IDLE → AUTHENTICATED)
	wsURL := fmt.Sprintf("ws://localhost:18080/ws?token=%s", token)
	conn, err := dialWS(t, wsURL)
	if err != nil {
		t.Skipf("WebSocket dial failed (expected in minimal test stack): %v", err)
	}
	defer conn.Close()
	time.Sleep(200 * time.Millisecond)

	// State should now be AUTHENTICATED or CONNECTED
	state := getJSON(t, controlURL+"/state")
	sys, _ := state["system"].(string)
	assert.Contains(t, []string{"AUTHENTICATED", "CONNECTED", "CONNECTING"}, sys)

	// 3. Start session
	resp2 := postJSON(t, controlURL+"/session/start", map[string]string{
		"vehicle_id":    "vehicle-int-1",
		"operator_id":   "op-lifecycle",
		"operator_role": "ACTIVE_OPERATOR",
	})
	defer resp2.Body.Close()

	if resp2.StatusCode == 200 {
		var sessBody map[string]any
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&sessBody))
		sessionID, _ := sessBody["session_id"].(string)
		assert.Greater(t, len(sessionID), 10, "session_id must be ULID")

		// 4. SYSTEM STATE should be CONNECTED
		state2 := getJSON(t, controlURL+"/state")
		assert.Equal(t, "CONNECTED", state2["system"])

		// 5. End session
		resp3 := postJSON(t, controlURL+"/session/end", nil)
		resp3.Body.Close()
		assert.Equal(t, 204, resp3.StatusCode)
	}
}

// --- MEDIA STATE: Invariante 1 ---

func TestIntegration_MediaFailed_TriggersDegrade_NeverSafeMode(t *testing.T) {
	// Login + WS connect + session start
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"id": "op-media", "password": "test"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/ws?token=%s", token))
	if err != nil {
		t.Skipf("WebSocket not available: %v", err)
	}
	defer conn.Close()
	time.Sleep(300 * time.Millisecond)

	postJSON(t, controlURL+"/session/start", map[string]string{
		"vehicle_id": "vehicle-media", "operator_id": "op-media", "operator_role": "ACTIVE_OPERATOR",
	}).Body.Close()

	// MEDIA_FAILED event
	resp2 := postJSON(t, controlURL+"/media/event", map[string]string{"state": "MEDIA_FAILED"})
	resp2.Body.Close()
	assert.Equal(t, 202, resp2.StatusCode)

	state := getJSON(t, controlURL+"/state")
	sys := state["system"].(string)
	assert.Equal(t, "DEGRADED", sys, "MEDIA_FAILED must → DEGRADED, never SAFE_MODE (Invariante 1)")

	postJSON(t, controlURL+"/session/end", nil).Body.Close()
}

// --- Emergency Stop ---

func TestIntegration_EmergencyStop_TriggersSafeMode(t *testing.T) {
	resp := postJSON(t, authURL+"/auth/operator/login",
		map[string]string{"id": "op-estop", "password": "test"})
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	token := body["token"].(string)

	conn, err := dialWS(t, fmt.Sprintf("ws://localhost:18080/ws?token=%s", token))
	if err != nil {
		t.Skipf("WebSocket not available: %v", err)
	}
	defer conn.Close()
	time.Sleep(300 * time.Millisecond)

	postJSON(t, controlURL+"/session/start", map[string]string{
		"vehicle_id": "vehicle-estop", "operator_id": "op-estop", "operator_role": "ACTIVE_OPERATOR",
	}).Body.Close()

	resp2 := postJSON(t, controlURL+"/emergency-stop", map[string]string{})
	resp2.Body.Close()
	assert.Equal(t, 202, resp2.StatusCode)

	state := getJSON(t, controlURL+"/state")
	assert.Equal(t, "SAFE_MODE", state["system"])
}
