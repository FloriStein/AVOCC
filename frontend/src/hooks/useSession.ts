import { useCallback, useRef, useState } from 'react'
import { login, startSession, endSession as endSessionAPI } from '@/lib/api-client'
import { WSClient } from '@/lib/ws-client'

const OPERATOR_ID = 'operator-1'

// Exponential backoff delays in ms (1s, 2s, 4s, 8s, max 30s).
const BACKOFF = [1000, 2000, 4000, 8000, 16000, 30000]

export interface SessionState {
  token: string | null
  sessionId: string | null
  vehicleId: string | null
  latency: number
  wsClient: WSClient
  connect: () => Promise<void>
  resume: () => Promise<void>
  disconnect: () => void
  startSession: (vehicleId: string) => Promise<void>
  endSession: () => Promise<void>
}

export function useSession(): SessionState {
  const [token, setToken] = useState<string | null>(null)
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [vehicleId, setVehicleId] = useState<string | null>(null)
  const [latency, setLatency] = useState(0)

  const wsClient = useRef(new WSClient()).current
  const reconnectAttempt = useRef(0)

  const connectWS = useCallback((t: string) => {
    wsClient.onAck = (ms) => setLatency(ms)
    wsClient.onClose = async () => {
      // Exponential backoff reconnect after unexpected server-side close (ADR-010).
      // Intentional closes via wsClient.disconnect() suppress this via ws.onclose = null.
      const delay = BACKOFF[Math.min(reconnectAttempt.current, BACKOFF.length - 1)]
      reconnectAttempt.current++
      await new Promise((r) => setTimeout(r, delay))
      wsClient.connect(t)
    }
    wsClient.connect(t)
  }, [wsClient])

  const connect = useCallback(async () => {
    reconnectAttempt.current = 0
    const t = await login(OPERATOR_ID)
    setToken(t)
    setSessionId(null)
    setVehicleId(null)
    connectWS(t)
  }, [connectWS])

  // Called by VehicleSelector when the operator clicks "Session starten".
  const startSessionFn = useCallback(async (vid: string) => {
    try {
      const sid = await startSession(vid, OPERATOR_ID)
      setSessionId(sid)
      setVehicleId(vid)
      reconnectAttempt.current = 0
    } catch {
      // caller can retry
    }
  }, [])

  // Called when operator deliberately ends a session to pick a different vehicle.
  // vehicleId is intentionally kept so VehicleSelector can pre-select it on remount.
  // Backend transitions to SAFE_MODE — operator must use Resume to return to AUTHENTICATED.
  const endSessionFn = useCallback(async () => {
    await endSessionAPI()
    setSessionId(null)
  }, [])

  // Resume after SAFE_MODE: silently close old WS (no onClose callback) and reconnect.
  const resume = useCallback(async () => {
    if (!token) return
    wsClient.disconnect()
    connectWS(token)
  }, [token, wsClient, connectWS])

  const disconnect = useCallback(() => {
    wsClient.disconnect()
    setToken(null)
    setSessionId(null)
    setVehicleId(null)
  }, [wsClient])

  return { token, sessionId, vehicleId, latency, wsClient, connect, resume, disconnect, startSession: startSessionFn, endSession: endSessionFn }
}
