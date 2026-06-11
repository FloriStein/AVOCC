package vehicleconnection

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// Registry tracks active vehicle WebSocket connections keyed by vehicleID (ADR-021).
// ForwardCommand implements command.VehicleForwarder — called by the Command Engine
// to deliver ControlCommand bytes to the connected vehicle.
type Registry struct {
	mu    sync.RWMutex
	conns map[string]*websocket.Conn
}

func NewRegistry() *Registry {
	return &Registry{conns: make(map[string]*websocket.Conn)}
}

func (r *Registry) register(vehicleID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[vehicleID] = conn
}

func (r *Registry) unregister(vehicleID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, vehicleID)
}

// ForwardCommand sends raw Protobuf bytes to the vehicle's WebSocket connection.
// Fire-and-forget: errors are returned but never block the Control path (ADR-021).
func (r *Registry) ForwardCommand(vehicleID string, data []byte) error {
	r.mu.RLock()
	conn, ok := r.conns[vehicleID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("vehicle %s not connected", vehicleID)
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

// Connected reports whether a vehicle is currently registered.
func (r *Registry) Connected(vehicleID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.conns[vehicleID]
	return ok
}

// RegisterForTest exposes register() for use in tests. Not for production call sites.
func (r *Registry) RegisterForTest(vehicleID string, conn *websocket.Conn) {
	r.register(vehicleID, conn)
}
