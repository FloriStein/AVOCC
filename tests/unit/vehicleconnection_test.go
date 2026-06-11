package unit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	vehiclev1 "avoc/gen/go/vehicle/v1"
	"avoc/internal/vehicleconnection"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// ─── Registry ─────────────────────────────────────────────────────────────────

func TestRegistry_ForwardCommand_NoVehicle_ReturnsError(t *testing.T) {
	r := vehicleconnection.NewRegistry()
	err := r.ForwardCommand("vehicle-99", []byte{0x01, 0x02})
	assert.Error(t, err, "forwarding to unregistered vehicle must fail")
}

func TestRegistry_Connected_FalseBeforeRegister(t *testing.T) {
	r := vehicleconnection.NewRegistry()
	assert.False(t, r.Connected("vehicle-001"))
}

func TestRegistry_ForwardCommand_DeliversBytesToVehicle(t *testing.T) {
	// Spin up a WebSocket echo server that records received messages.
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		require.NoError(t, err)
		received <- data
	}))
	defer srv.Close()

	// Connect a WebSocket client and register it in the Registry.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	r := vehicleconnection.NewRegistry()
	r.RegisterForTest("vehicle-001", conn)

	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	require.NoError(t, r.ForwardCommand("vehicle-001", payload))

	select {
	case got := <-received:
		assert.Equal(t, payload, got)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: vehicle did not receive forwarded command")
	}
}

// ─── AckStore ─────────────────────────────────────────────────────────────────

func TestAckStore_Latest_EmptyReturnsNotFound(t *testing.T) {
	s := vehicleconnection.NewAckStore()
	_, ok := s.Latest("vehicle-001")
	assert.False(t, ok)
}

func TestAckStore_StoreAndRetrieve(t *testing.T) {
	s := vehicleconnection.NewAckStore()
	ack := &vehiclev1.VehicleCommandAck{
		CommandEventId: "EVT-001",
		Received:       true,
		ReceivedAtMs:   12345,
	}
	s.Store("vehicle-001", ack)

	got, ok := s.Latest("vehicle-001")
	require.True(t, ok)
	assert.True(t, proto.Equal(ack, got))
}

func TestAckStore_OverwritesWithLatest(t *testing.T) {
	s := vehicleconnection.NewAckStore()
	s.Store("vehicle-001", &vehiclev1.VehicleCommandAck{CommandEventId: "EVT-001"})
	s.Store("vehicle-001", &vehiclev1.VehicleCommandAck{CommandEventId: "EVT-002"})

	got, ok := s.Latest("vehicle-001")
	require.True(t, ok)
	assert.Equal(t, "EVT-002", got.CommandEventId)
}

func TestAckStore_IsolatedPerVehicle(t *testing.T) {
	s := vehicleconnection.NewAckStore()
	s.Store("vehicle-001", &vehiclev1.VehicleCommandAck{CommandEventId: "A"})
	s.Store("vehicle-002", &vehiclev1.VehicleCommandAck{CommandEventId: "B"})

	a, _ := s.Latest("vehicle-001")
	b, _ := s.Latest("vehicle-002")
	assert.Equal(t, "A", a.CommandEventId)
	assert.Equal(t, "B", b.CommandEventId)
}
