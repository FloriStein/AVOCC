import { useCallback, useRef, useState } from 'react'
import { login, startSession, endSession as endSessionAPI } from '@/lib/api-client'
import { WSClient } from '@/lib/ws-client'

// Exponential backoff delays in ms (1s, 2s, 4s, 8s, max 30s).
const BACKOFF = [1000, 2000, 4000, 8000, 16000, 30000]

export interface SessionState {
  token: string | null
  operatorId: string | null
  sessionId: string | null
  vehicleId: string | null
  latency: number
  wsClient: WSClient
  connect: (id: string, password: string) => Promise<void>
  resume: () => Promise<void>
  disconnect: () => void
  startSession: (vehicleId: string) => Promise<void>
  endSession: () => Promise<void>
}

export function useSession(): SessionState {
  const [token, setToken] = useState<string | null>(null)
  const [operatorId, setOperatorId] = useState<string | null>(null)
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [vehicleId, setVehicleId] = useState<string | null>(null)
  const [latency, setLatency] = useState(0)

  const wsClient = useRef(new WSClient()).current
  const reconnectAttempt = useRef(0)
  const tokenRef = useRef<string | null>(null)

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

  const connect = useCallback(async (id: string, password: string) => {
    reconnectAttempt.current = 0
    const t = await login(id, password)
    tokenRef.current = t
    setToken(t)
    setOperatorId(id)
    setSessionId(null)
    setVehicleId(null)
    connectWS(t)
  }, [connectWS])

  // Called by VehicleSelector when the operator clicks "Session starten".
  const startSessionFn = useCallback(async (vid: string) => {
    const t = tokenRef.current
    const opId = operatorId
    if (!t || !opId) return
    try {
      const sid = await startSession(vid, opId, t)
      setSessionId(sid)
      setVehicleId(vid)
      reconnectAttempt.current = 0
    } catch {
      // caller can retry
    }
  }, [operatorId])

  // Called when operator deliberately ends a session to pick a different vehicle.
  const endSessionFn = useCallback(async () => {
    const t = tokenRef.current
    if (!t) return
    await endSessionAPI(t)
    setSessionId(null)
  }, [])

  // Resume after SAFE_MODE: silently close old WS (no onClose callback) and reconnect.
  const resume = useCallback(async () => {
    const t = tokenRef.current
    if (!t) return
    wsClient.disconnect()
    connectWS(t)
  }, [wsClient, connectWS])

  const disconnect = useCallback(() => {
    wsClient.disconnect()
    tokenRef.current = null
    setToken(null)
    setOperatorId(null)
    setSessionId(null)
    setVehicleId(null)
  }, [wsClient])

  return { token, operatorId, sessionId, vehicleId, latency, wsClient, connect, resume, disconnect, startSession: startSessionFn, endSession: endSessionFn }
}
