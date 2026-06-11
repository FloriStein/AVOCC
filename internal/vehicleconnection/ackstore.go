package vehicleconnection

import (
	"sync"

	vehiclev1 "avoc/gen/go/vehicle/v1"
)

// AckStore holds the most recent VehicleCommandAck per vehicle (ADR-021).
// Exposed via GET /vehicle/ack/latest/{vehicleId} for frontend polling.
type AckStore struct {
	mu   sync.RWMutex
	acks map[string]*vehiclev1.VehicleCommandAck
}

func NewAckStore() *AckStore {
	return &AckStore{acks: make(map[string]*vehiclev1.VehicleCommandAck)}
}

func (s *AckStore) Store(vehicleID string, ack *vehiclev1.VehicleCommandAck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acks[vehicleID] = ack
}

func (s *AckStore) Latest(vehicleID string) (*vehiclev1.VehicleCommandAck, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ack, ok := s.acks[vehicleID]
	return ack, ok
}
