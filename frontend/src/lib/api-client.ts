// HTTP client for AVOC backend REST endpoints (via nginx proxy).
// All paths are relative — nginx routes /api/ → control-server, /auth/ → auth-service.

export interface SystemStateResponse {
  system: string
  control: string
  media: string
  operator: string
  session_id?: string
}

export async function login(operatorId: string): Promise<string> {
  const res = await fetch('/auth/operator/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: operatorId, password: 'test' }),
  })
  if (!res.ok) throw new Error(`login failed: ${res.status}`)
  const { token } = await res.json()
  return token as string
}

export async function getState(): Promise<SystemStateResponse> {
  const res = await fetch('/api/state')
  if (!res.ok) throw new Error(`getState failed: ${res.status}`)
  return res.json()
}

export async function startSession(vehicleId: string, operatorId: string): Promise<string> {
  const res = await fetch('/api/session/start', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      vehicle_id: vehicleId,
      operator_id: operatorId,
      operator_role: 'ACTIVE_OPERATOR',
    }),
  })
  if (!res.ok) throw new Error(`startSession failed: ${res.status}`)
  const { session_id } = await res.json()
  return session_id as string
}

export async function endSession(): Promise<void> {
  await fetch('/api/session/end', { method: 'POST' })
}

export async function emergencyStop(sessionId: string, vehicleId: string): Promise<void> {
  await fetch('/api/emergency-stop', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      session_id: sessionId,
      vehicle_id: vehicleId,
      reason: 'operator emergency stop',
    }),
  })
}

// Reports WebRTC MEDIA STATE changes to the Control Server (ADR-009 Invariant 1).
// MEDIA_FAILED → DEGRADED on server side — never SAFE_MODE.
export async function reportMediaState(state: string): Promise<void> {
  await fetch('/api/media/event', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ state }),
  })
}
