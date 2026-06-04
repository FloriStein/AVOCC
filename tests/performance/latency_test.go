// Package performance measures ACK-Roundtrip latency for the Control Loop (ADR-006/010).
// Run against the test stack: make test-latency
// CI Build-Fail when p99 > 100ms (ADR-010: <100ms hard requirement).
package performance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	testAuthURL    = "http://localhost:18081"
	testControlURL = "http://localhost:18080"
	testJWTSecret  = "test-secret-integration"
	latencyBudget  = 100 * time.Millisecond // ADR-010 hard requirement
)

func loginOperator(t testing.TB, id string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"id": id, "password": "test"})
	resp, err := http.Post(testAuthURL+"/auth/operator/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("auth login failed: %v", err)
	}
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	token, _ := m["token"].(string)
	return token
}

func startSession(t testing.TB, vehicleID, operatorID string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"vehicle_id":    vehicleID,
		"operator_id":   operatorID,
		"operator_role": "ACTIVE_OPERATOR",
	})
	http.Post(testControlURL+"/session/start", "application/json", bytes.NewReader(body))
}

// BenchmarkControlACKRoundtrip measures the WebSocket ACK roundtrip for a
// Protobuf DEADMAN_HOLD command (field 2 = type 6, minimal valid message).
// CI Build-Fail: p99 must stay < 100ms (ADR-010).
func BenchmarkControlACKRoundtrip(b *testing.B) {
	token := loginOperator(b, "bench-operator")
	if token == "" {
		b.Skip("auth service not available — start test stack with: make test-integration")
	}

	wsURL := fmt.Sprintf("ws://localhost:18080/ws?token=%s", token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Skipf("WebSocket not available (start test stack): %v", err)
	}
	defer conn.Close()

	time.Sleep(300 * time.Millisecond)
	startSession(b, "bench-vehicle", "bench-operator")
	time.Sleep(100 * time.Millisecond)

	// Minimal Protobuf ControlCommand: field 2 (type=DEADMAN_HOLD=6) as varint
	cmdDeadmanHold := []byte{0x10, 0x06}

	latencies := make([]time.Duration, 0, b.N)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		if err := conn.WriteMessage(websocket.BinaryMessage, cmdDeadmanHold); err != nil {
			b.Fatalf("write error: %v", err)
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			b.Fatalf("read ACK error: %v", err)
		}
		latencies = append(latencies, time.Since(t0))
	}

	b.StopTimer()

	// Compute percentiles
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[int(math.Min(float64(len(latencies)*99/100), float64(len(latencies)-1)))]

	b.ReportMetric(float64(p50.Milliseconds()), "p50_ms")
	b.ReportMetric(float64(p95.Milliseconds()), "p95_ms")
	b.ReportMetric(float64(p99.Milliseconds()), "p99_ms")

	// CI Build-Fail guard — ADR-010: <100ms is non-negotiable
	if p99 > latencyBudget {
		b.Fatalf("LATENCY BUDGET EXCEEDED: p99=%v > %v (ADR-010)", p99, latencyBudget)
	}

	b.Logf("ACK Roundtrip — p50=%v p95=%v p99=%v (budget=%v) ✅",
		p50.Round(time.Millisecond),
		p95.Round(time.Millisecond),
		p99.Round(time.Millisecond),
		latencyBudget)
}

// TestLatencyBudget_DocumentedRequirement verifies the budget constant matches ADR-010.
func TestLatencyBudget_DocumentedRequirement(t *testing.T) {
	if latencyBudget != 100*time.Millisecond {
		t.Fatalf("latency budget must be 100ms per ADR-010, got %v", latencyBudget)
	}
}
