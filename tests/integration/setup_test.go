// Package integration tests all services in the Docker test stack (TEST-03, ADR-006).
// Run via: make test-integration (starts/stops docker-compose.test.yml automatically).
// Requires the test stack to be running: ports 18080 (control), 18081 (auth), 18082 (safety).
package integration_test

import (
	"os"
	"testing"
)

const (
	controlURL = "http://localhost:18080"
	authURL    = "http://localhost:18081"
	safetyURL  = "http://localhost:18082"
	jwtSecret  = "test-secret-integration"
)

// TestMain allows suite-level setup/teardown if needed.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
