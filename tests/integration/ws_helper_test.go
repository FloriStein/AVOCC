package integration_test

import (
	"testing"

	"github.com/gorilla/websocket"
)

// dialWS opens a WebSocket connection to the given URL.
// Returns the connection or an error (caller must decide whether to skip or fail).
func dialWS(t *testing.T, url string) (*websocket.Conn, error) {
	t.Helper()
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	return conn, err
}
