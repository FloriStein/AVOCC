import { useCallback, useEffect, useRef, useState } from 'react'
import { reportMediaState } from '@/lib/api-client'
import { FE_WEBRTC_STATE, logEvent } from '@/lib/logger'

export type MediaState =
  | 'MEDIA_INIT'
  | 'MEDIA_NEGOTIATING'
  | 'MEDIA_CONNECTED'
  | 'MEDIA_DEGRADED'
  | 'MEDIA_FAILED'

export function useWebRTC(sessionId: string | null, vehicleId: string, enabled: boolean) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const [mediaState, setMediaState] = useState<MediaState>('MEDIA_INIT')
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const tokenRef = useRef<string | null>(null)

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
    setMediaState('MEDIA_INIT')
  }, [])

  const connect = useCallback(async () => {
    if (!sessionId || !vehicleId) return
    disconnect()
    updateState('MEDIA_NEGOTIATING')

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: `stun:${window.location.hostname}:3479` }],
    })
    pcRef.current = pc

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

      // WHEP: ICE-Gathering vollständig abwarten (non-trickle)
      await new Promise<void>(resolve => {
        if (pc.iceGatheringState === 'complete') { resolve(); return }
        pc.addEventListener('icegatheringstatechange', function handler() {
          if (pc.iceGatheringState === 'complete') {
            pc.removeEventListener('icegatheringstatechange', handler)
            resolve()
          }
        })
      })

      // WHEP: raw SDP als Body, Content-Type application/sdp (ADR-020)
      const res = await fetch(`/whep/${vehicleId}/whep`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/sdp',
          ...(tokenRef.current ? { 'Authorization': `Bearer ${tokenRef.current}` } : {}),
        },
        body: pc.localDescription!.sdp,
      })

      if (!res.ok) { updateState('MEDIA_FAILED'); return }

      const answerSdp = await res.text()
      await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
    } catch {
      updateState('MEDIA_FAILED')
    }
  }, [sessionId, vehicleId, disconnect, updateState])

  useEffect(() => {
    if (enabled && sessionId) {
      connect()
    } else {
      disconnect()
    }
    return disconnect
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, sessionId, vehicleId])

  return { videoRef, mediaState, connect, disconnect, tokenRef }
}
