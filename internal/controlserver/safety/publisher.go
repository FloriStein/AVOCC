// Package safety implements the Control Server's Safety Decision Module (ADR-009).
// It classifies failures as CRITICAL/DEGRADED and enforces the three system invariants.
package safety

import "avoc/internal/safetyservice"

// Publisher sends safety events to the external Safety Event Bus service (ADR-002).
// Implemented by HTTPPublisher in production and MockSafetyPublisher in tests.
type Publisher interface {
	TriggerEmergencyStop(sessionID, vehicleID, reason string)
	PublishEvent(event safetyservice.SafetyEvent)
}
