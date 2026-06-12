import { useCallback, useEffect, useRef, useState } from 'react'
import { reportMediaState } from '@/lib/api-client'
import { FE_WEBRTC_STATE, logEvent } from '@/lib/logger'

export type MediaState =
  | 'MEDIA_INIT'
  | 'MEDIA_NEGOTIATING'
  | 'MEDIA_CONNECTED'
  | 'MEDIA_DEGRADED'
  | 'MEDIA_FAILED'

function isValidIceServer(s: RTCIceServer): boolean {
  const urls = Array.isArray(s.urls) ? s.urls : [s.urls]
  // Reject entries where the URL contains an empty host (e.g. "stun::3478" when TURN_EXTERNAL_IP is unset)
  return urls.every(u => !/^(stun|turn):(?::|\?)/.test(u))
}

async function fetchIceServers(): Promise<RTCIceServer[]> {
  try {
    const res = await fetch('/api/ice-config')
    if (!res.ok) throw new Error(`ice-config ${res.status}`)
    const { iceServers } = await res.json()
    const valid = (iceServers as RTCIceServer[]).filter(isValidIceServer)
    if (valid.length > 0) return valid
    throw new Error('no valid ICE servers in response')
  } catch {
    // Fallback: STUN only — works on non-CGNAT networks (WiFi/DSL)
    const host = window.location.hostname || '127.0.0.1'
    return [{ urls: `stun:${host}:3478` }]
  }
}

export function useWebRTC(sessionId: string | null, vehicleId: string, token: string | null, enabled: boolean) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const [mediaState, setMediaState] = useState<MediaState>('MEDIA_INIT')
  const pcRef = useRef<RTCPeerConnection | null>(null)

  const updateState = useCallback((state: MediaState) => {
    setMediaState(state)
    reportMediaState(state).catch(() => {})
    logEvent(FE_WEBRTC_STATE, 'WebRTC media state changed',
      { sessionId: sessionId ?? '', data: { state } })
  }, [sessionId])

  const disconnect = useCallback(() => {
    if (pcRef.current) {
      pcRef.current.close()
      pcRef.current = null
    }
    if (videoRef.current) videoRef.current.srcObject = null
    streamRef.current = null
    setMediaState('MEDIA_INIT')
  }, [])

  const connect = useCallback(async () => {
    if (!sessionId || !vehicleId || !token) return
    disconnect()
    updateState('MEDIA_NEGOTIATING')

    const iceServers = await fetchIceServers()
    const pc = new RTCPeerConnection({ iceServers })
    pcRef.current = pc

    pc.addTransceiver('video', { direction: 'recvonly' })

    pc.ontrack = (event) => {
      if (videoRef.current && event.streams.length > 0) {
        videoRef.current.srcObject = event.streams[0]
        streamRef.current = event.streams[0]
        updateState('MEDIA_CONNECTED')
      }
    }

    pc.oniceconnectionstatechange = () => {
      if (pc !== pcRef.current) return
      if (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected') {
        updateState('MEDIA_FAILED')
      }
    }

    try {
      const offer = await pc.createOffer()
      // Guard: verhindert Race Condition durch React StrictMode double-mount.
      // Wenn eine neuere connect()-Instanz unsere PC ersetzt hat, still abbrechen.
      if (pc !== pcRef.current) return

      // Pion v1.19.0 DTLS-Client-Bug: nach dem Senden von ClientHello verarbeitet Pion
      // den ServerHello des Browsers nicht → Retransmit-Loop bis Timeout.
      // Fix: Browser wird DTLS-Client (active), MediaMTX wird DTLS-Server (passive).
      // Pions DTLS-Server-Pfad ist stabil; nur der Client-Pfad hat diesen Bug.
      const fixedSdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active')
      await pc.setLocalDescription({ type: 'offer', sdp: fixedSdp })
      if (pc !== pcRef.current) return

      // WHEP: alle ICE-Candidates vollständig abwarten bevor der Offer gesendet wird.
      // MediaMTX erwartet alle Candidates im initialen POST (kein Trickle-ICE).
      await new Promise<void>(resolve => {
        if (pc.iceGatheringState === 'complete') { resolve(); return }
        const tid = setTimeout(resolve, 2000)
        pc.addEventListener('icegatheringstatechange', function handler() {
          if (pc.iceGatheringState === 'complete') {
            clearTimeout(tid)
            pc.removeEventListener('icegatheringstatechange', handler)
            resolve()
          }
        })
      })
      if (pc !== pcRef.current) return

      const res = await fetch(`/whep/${vehicleId}/whep`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/sdp',
          'Authorization': `Bearer ${token}`,
        },
        body: pc.localDescription!.sdp,
      })
      if (pc !== pcRef.current) return

      if (!res.ok) { updateState('MEDIA_FAILED'); return }

      const answerSdp = await res.text()
      if (pc !== pcRef.current) return
      await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
    } catch {
      if (pc === pcRef.current) updateState('MEDIA_FAILED')
    }
  }, [sessionId, vehicleId, token, disconnect, updateState])

  useEffect(() => {
    if (enabled && sessionId && token) {
      connect()
    } else {
      disconnect()
    }
    return disconnect
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, sessionId, vehicleId, token])

  // Auto-retry when stream not yet available (MEDIA_FAILED + enabled)
  useEffect(() => {
    if (mediaState !== 'MEDIA_FAILED' || !enabled || !sessionId || !token) return
    const tid = setTimeout(connect, 3000)
    return () => clearTimeout(tid)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mediaState, enabled, sessionId, token])

  return { videoRef, streamRef, mediaState, connect, disconnect }
}
