package logger

// Event type constants for all structured log events (ADR-017).
// Use with Logger.Event() so Loki/Grafana can filter by event_type label.
const (
	// Session lifecycle
	EventSessionStarted = "SESSION_STARTED"
	EventSessionEnded   = "SESSION_ENDED"

	// Safety critical events — these also write to SQLite via AuditWriter (ADR-018)
	EventSafeModeEntered  = "SAFE_MODE_ENTERED"
	EventEmergencyStop    = "EMERGENCY_STOP"
	EventDeadmanTimeout   = "DEADMAN_TIMEOUT"
	EventDeadmanArmed     = "DEADMAN_ARMED"
	EventAckTimeout       = "COMMAND_ACK_TIMEOUT"
	EventWsDisconnect     = "WS_DISCONNECT_CRITICAL"
	EventOperatorHandover = "OPERATOR_HANDOVER_COMPLETED"

	// System events
	EventStateTransition  = "STATE_TRANSITION_SYSTEM"
	EventCommandReceived  = "COMMAND_RECEIVED"
	EventMediaStateChange = "MEDIA_STATE_CHANGE"

	// Frontend events — received via POST /log (LOG-07)
	EventFEEmergencyStop = "FE_EMERGENCY_STOP_CLICKED"
	EventFEDeadmanHold   = "FE_DEADMAN_HOLD"
	EventFEWebRTCState   = "FE_WEBRTC_STATE_CHANGE"
	EventFEWSReconnect   = "FE_WS_RECONNECT"
	EventFEOperatorAck   = "FE_OPERATOR_ACK_CLICKED"
)
