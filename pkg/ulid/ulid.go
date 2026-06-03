// Package ulid wraps oklog/ulid/v2 for use across the system (ADR-016).
// All Session-IDs and Event-IDs are ULIDs — time-sortable, URL-safe, distributed-safe.
package ulid

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// Generate returns a new ULID string.
// Safe for concurrent use — uses crypto/rand as entropy source.
func Generate() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}
