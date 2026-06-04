// Frontend structured logger — sends events to Control Server via POST /api/log.
// The Control Server forwards them to slog (service="frontend") → Loki (ADR-017, LOG-08).
// All calls are fire-and-forget — a failed log POST never blocks the UI.

interface LogContext {
  sessionId?: string
  vehicleId?: string
  operatorId?: string
  eventId?: string
  data?: Record<string, unknown>
}

// logEvent sends a structured event to the backend log ingestion endpoint.
// eventType should be one of the FE_* constants below.
export function logEvent(
  eventType: string,
  msg: string,
  context: LogContext = {},
): void {
  const body = JSON.stringify({
    level: 'info',
    event_type: eventType,
    msg,
    session_id: context.sessionId ?? '',
    vehicle_id: context.vehicleId ?? '',
    operator_id: context.operatorId ?? '',
    event_id: context.eventId ?? '',
    data: context.data,
  })

  // Fire-and-forget — errors are swallowed intentionally (log loss is acceptable, ADR-017)
  fetch('/api/log', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
    keepalive: true, // survives page unload
  }).catch(() => {
    // no-op: log POST failure must never surface to the operator
  })
}

// Frontend event type constants (ADR-017)
export const FE_EMERGENCY_STOP  = 'FE_EMERGENCY_STOP_CLICKED'
export const FE_DEADMAN_HOLD    = 'FE_DEADMAN_HOLD'
export const FE_WEBRTC_STATE    = 'FE_WEBRTC_STATE_CHANGE'
export const FE_WS_RECONNECT    = 'FE_WS_RECONNECT'
export const FE_OPERATOR_ACK    = 'FE_OPERATOR_ACK_CLICKED'
