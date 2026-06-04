import { useCallback, useEffect, useRef, useState } from 'react'
import { reportMediaState } from '@/lib/api-client'

const OPERATOR_PEER_ID = 'operator-1'

export type MediaState =
  | 'MEDIA_INIT'
  | 'MEDIA_NEGOTIATING'
  | 'MEDIA_CONNECTED'
  | 'MEDIA_DEGRADED'
  | 'MEDIA_FAILED'

export function useWebRTC(sessionId: string | null, enabled: boolean) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [mediaState, setMediaState] = useState<MediaState>('MEDIA_INIT')
  const pcRef = useRef<RTCPeerConnection | null>(null)

  const updateState = useCallback((state: MediaState) => {
    setMediaState(state)
    // Notify Control Server — MEDIA_FAILED → DEGRADED (never SAFE_MODE, ADR-009 Invariant 1)
    reportMediaState(state).catch(() => {})
  }, [])

  const disconnect = useCallback(() => {
    if (pcRef.current) {
      pcRef.current.close()
      pcRef.current = null
    }
    if (videoRef.current) videoRef.current.srcObject = null
    setMediaState('MEDIA_INIT')
  }, [])

  const connect = useCallback(async () => {
    if (!sessionId) return
    disconnect()
    updateState('MEDIA_NEGOTIATING')

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: `stun:${window.location.hostname}:3479` }],
    })
    pcRef.current = pc

    // Browser receives video from SFU (recvonly — vehicle sends, SFU forwards)
    pc.addTransceiver('video', { direction: 'recvonly' })

    pc.ontrack = (event) => {
      if (videoRef.current && event.streams.length > 0) {
        videoRef.current.srcObject = event.streams[0]
        updateState('MEDIA_CONNECTED')
      }
    }

    pc.oniceconnectionstatechange = () => {
      if (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected') {
        updateState('MEDIA_FAILED')
      }
    }

    try {
      const offer = await pc.createOffer()
      await pc.setLocalDescription(offer)

      const res = await fetch(`/sfu/subscribe/${sessionId}/${OPERATOR_PEER_ID}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sdp: offer.sdp }),
      })

      if (!res.ok) { updateState('MEDIA_FAILED'); return }

      const { sdp } = await res.json() as { sdp: string }
      await pc.setRemoteDescription({ type: 'answer', sdp })
    } catch {
      updateState('MEDIA_FAILED')
    }
  }, [sessionId, disconnect, updateState])

  // Auto-connect when session becomes active; disconnect when session ends or disabled
  useEffect(() => {
    if (enabled && sessionId) {
      connect()
    } else {
      disconnect()
    }
    return disconnect
  // connect/disconnect are stable useCallback refs — sessionId change is intentional trigger
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, sessionId])

  return { videoRef, mediaState, connect, disconnect }
}
