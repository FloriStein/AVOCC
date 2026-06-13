// HTTP client for AVOC backend REST endpoints (via nginx proxy).
// All paths are relative — nginx routes /api/ → control-server, /auth/ → auth-service.

export interface SystemStateResponse {
  system: string
  control: string
  media: string
  operator: string
  session_id?: string
}

export async function login(id: string, password: string): Promise<string> {
  const res = await fetch('/auth/operator/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id, password }),
  })
  if (!res.ok) throw new Error(`login failed: ${res.status}`)
  const { token } = await res.json()
  return token as string
}

// Decodes the JWT payload and returns the `role` claim without an external library.
export function parseTokenRole(token: string): string {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return payload.role ?? ''
  } catch {
    return ''
  }
}

export async function getState(): Promise<SystemStateResponse> {
  const res = await fetch('/api/state')
  if (!res.ok) throw new Error(`getState failed: ${res.status}`)
  return res.json()
}

export async function startSession(vehicleId: string, operatorId: string, token: string): Promise<string> {
  const res = await fetch('/api/session/start', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
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

export async function endSession(token: string): Promise<void> {
  await fetch('/api/session/end', {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${token}` },
  })
}

export async function emergencyStop(sessionId: string, vehicleId: string, token: string): Promise<void> {
  await fetch('/api/emergency-stop', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({
      session_id: sessionId,
      vehicle_id: vehicleId,
      reason: 'operator emergency stop',
    }),
  })
}

export interface VehicleInfo {
  id: string
  display_name: string
  description: string
  online: boolean
}

export async function listVehicles(): Promise<VehicleInfo[]> {
  const res = await fetch('/api/vehicles')
  if (!res.ok) throw new Error(`listVehicles failed: ${res.status}`)
  return res.json()
}

// Reports WebRTC MEDIA STATE changes to the Control Server (ADR-009 Invariant 1).
// MEDIA_FAILED → DEGRADED on server side — never SAFE_MODE.
export async function reportMediaState(state: string, token: string): Promise<void> {
  await fetch('/api/media/event', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ state }),
  })
}

// ─── User Management (ADR-024) ───────────────────────────────────────────────

export interface UserInfo {
  id: string
  display_name: string
  role: string
  is_active: boolean
  created_at: string
  last_auth_at?: string
}

export async function listUsers(token: string): Promise<UserInfo[]> {
  const res = await fetch('/auth/users', {
    headers: { 'Authorization': `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(`listUsers failed: ${res.status}`)
  return res.json()
}

export async function createUser(
  token: string,
  id: string,
  displayName: string,
  password: string,
  role: string,
): Promise<void> {
  const res = await fetch('/auth/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ id, display_name: displayName, password, role }),
  })
  if (!res.ok) throw new Error(`createUser failed: ${res.status}`)
}

export async function deleteUser(token: string, id: string): Promise<void> {
  const res = await fetch(`/auth/users/${id}`, {
    method: 'DELETE',
    headers: { 'Authorization': `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(`deleteUser failed: ${res.status}`)
}

export async function updateUserRole(token: string, id: string, role: string): Promise<void> {
  const res = await fetch(`/auth/users/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
    body: JSON.stringify({ role }),
  })
  if (!res.ok) throw new Error(`updateUserRole failed: ${res.status}`)
}
