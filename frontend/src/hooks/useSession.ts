import { useCallback, useRef, useState } from 'react'
import { login, startSession } from '@/lib/api-client'
import { WSClient } from '@/lib/ws-client'

const OPERATOR_ID = 'operator-1'
const VEHICLE_ID = 'vehicle-001'

// Exponential backoff delays in ms (1s, 2s, 4s, 8s, max 30s).
const BACKOFF = [1000, 2000, 4000, 8000, 16000, 30000]

export interface SessionState {
  token: string | null
  sessionId: string | null
  vehicleId: string
  latency: number
  wsClient: WSClient
  connect: () => Promise<void>
  resume: () => Promise<void>
  disconnect: () => void
  startSessionIfNeeded: () => Promise<void>
}

export function useSession(): SessionState {
  const [token, setToken] = useState<string | null>(null)
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [latency, setLatency] = useState(0)

  const wsClient = useRef(new WSClient()).current
  const sessionStartedRef = useRef(false)
  const reconnectAttempt = useRef(0)

  const connectWS = useCallback((t: string) => {
    wsClient.onAck = (ms) => setLatency(ms)
    wsClient.onClose = async () => {
      // Exponential backoff reconnect after Channel Close (ADR-010)
      const delay = BACKOFF[Math.min(reconnectAttempt.current, BACKOFF.length - 1)]
      reconnectAttempt.current++
      await new Promise((r) => setTimeout(r, delay))
      wsClient.connect(t)
    }
    wsClient.connect(t)
  }, [wsClient])

  const connect = useCallback(async () => {
    sessionStartedRef.current = false
    reconnectAttempt.current = 0
    const t = await login(OPERATOR_ID)
    setToken(t)
    setSessionId(null)
    connectWS(t)
  }, [connectWS])

  // Called from App when SYSTEM STATE becomes AUTHENTICATED.
  const startSessionIfNeeded = useCallback(async () => {
    if (sessionStartedRef.current) return
    sessionStartedRef.current = true
    try {
      const sid = await startSession(VEHICLE_ID, OPERATOR_ID)
      setSessionId(sid)
      reconnectAttempt.current = 0
    } catch {
      sessionStartedRef.current = false
    }
  }, [])

  // Resume after SAFE_MODE — full reconnect flow.
  const resume = useCallback(async () => {
    sessionStartedRef.current = false
    wsClient.disconnect()
    if (!token) return
    connectWS(token)
  }, [token, wsClient, connectWS])

  const disconnect = useCallback(() => {
    wsClient.disconnect()
    setToken(null)
    setSessionId(null)
    sessionStartedRef.current = false
  }, [wsClient])

  return { token, sessionId, vehicleId: VEHICLE_ID, latency, wsClient, connect, resume, disconnect, startSessionIfNeeded }
}
