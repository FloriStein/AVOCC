import { useCallback, useEffect, useRef, useState } from 'react'

export type SenderStatus = 'idle' | 'gathering' | 'connecting' | 'live' | 'error' | 'stopped'
export type SourceType = 'camera' | 'screen'

function isValidIceServer(s: RTCIceServer): boolean {
  const urls = Array.isArray(s.urls) ? s.urls : [s.urls]
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
    const host = window.location.hostname || '127.0.0.1'
    return [{ urls: `stun:${host}:3478` }]
  }
}

export function useWHIPSender(previewRef: React.RefObject<HTMLVideoElement | null>) {
  const [status,     setStatus]     = useState<SenderStatus>('idle')
  const [error,      setError]      = useState<string | null>(null)
  const [candidates, setCandidates] = useState<RTCIceCandidate[]>([])

  const pcRef       = useRef<RTCPeerConnection | null>(null)
  const streamRef   = useRef<MediaStream | null>(null)
  const locationRef = useRef<string | null>(null)

  const stop = useCallback(() => {
    if (locationRef.current) {
      fetch(locationRef.current, { method: 'DELETE' }).catch(() => {})
      locationRef.current = null
    }
    pcRef.current?.close()
    pcRef.current = null
    streamRef.current?.getTracks().forEach(t => t.stop())
    streamRef.current = null
    if (previewRef.current) previewRef.current.srcObject = null
    setStatus('stopped')
    setError(null)
    setCandidates([])
  }, [previewRef])

  const start = useCallback(async (whipUrl: string, sourceType: SourceType, streamKey: string) => {
    stop()
    setStatus('gathering')
    setError(null)
    setCandidates([])

    try {
      let stream: MediaStream
      if (sourceType === 'screen') {
        stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true })
      } else {
        try {
          stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true })
        } catch {
          // Fallback: video only (microphone blocked or unavailable)
          stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false })
        }
      }

      streamRef.current = stream
      if (previewRef.current) previewRef.current.srcObject = stream

      const iceServers = await fetchIceServers()
      const pc = new RTCPeerConnection({ iceServers })
      pcRef.current = pc

      pc.onicecandidate = (e) => {
        if (e.candidate) setCandidates(prev => [...prev, e.candidate!])
      }

      pc.onconnectionstatechange = () => {
        const s = pc.connectionState
        if (s === 'connected')                       setStatus('live')
        if (s === 'failed')                          { setStatus('error'); setError('ICE-Verbindung fehlgeschlagen') }
        if (s === 'disconnected' || s === 'closed')  setStatus('stopped')
      }

      stream.getTracks().forEach(track => pc.addTrack(track, stream))

      const offer = await pc.createOffer()
      // Pion DTLS-Client-Bug-Fix: Browser wird DTLS-Client (active), MediaMTX DTLS-Server (passive)
      const sdp = offer.sdp!.replace(/a=setup:actpass/g, 'a=setup:active')
      await pc.setLocalDescription({ type: 'offer', sdp })

      // ICE-Gathering abwarten (max 5s) — MediaMTX erwartet alle Candidates im initialen WHIP-POST
      await new Promise<void>(resolve => {
        if (pc.iceGatheringState === 'complete') { resolve(); return }
        const tid = setTimeout(resolve, 2000)
        pc.onicegatheringstatechange = () => {
          if (pc.iceGatheringState === 'complete') { clearTimeout(tid); resolve() }
        }
      })

      setStatus('connecting')

      const res = await fetch(whipUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/sdp',
          'Authorization': `Bearer ${streamKey}`,
        },
        body: pc.localDescription!.sdp,
      })

      if (!res.ok) {
        const body = await res.text()
        throw new Error(`WHIP ${res.status}: ${body.trim() || res.statusText}`)
      }

      // MediaMTX antwortet mit einem relativen Location-Pfad (z.B. /vehicle-001/whip/abc123).
      // Für DELETE muss dieser Pfad durch den nginx/vite-Proxy (/whip/) geleitet werden.
      const location = res.headers.get('Location')
      if (location) {
        locationRef.current = location.startsWith('http')
          ? location
          : `/whip${location.startsWith('/') ? '' : '/'}${location}`
      }

      await pc.setRemoteDescription({ type: 'answer', sdp: await res.text() })

    } catch (err) {
      // Cleanup resources without calling stop() — stop() would overwrite 'error' status and clear the message
      pcRef.current?.close()
      pcRef.current = null
      streamRef.current?.getTracks().forEach(t => t.stop())
      streamRef.current = null
      if (previewRef.current) previewRef.current.srcObject = null
      setCandidates([])
      setStatus('error')
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [previewRef, stop])

  useEffect(() => () => { stop() }, [stop])

  return { status, error, candidates, start, stop }
}
