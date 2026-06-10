import { useWebRTC } from '@/hooks/useWebRTC'

interface Props {
  sessionId: string | null
  vehicleId: string
  token: string | null
  enabled: boolean
}

const MEDIA_BADGE: Record<string, string> = {
  MEDIA_INIT:        'bg-gray-600',
  MEDIA_NEGOTIATING: 'bg-blue-500 animate-pulse',
  MEDIA_CONNECTED:   'bg-green-500',
  MEDIA_DEGRADED:    'bg-yellow-500',
  MEDIA_FAILED:      'bg-red-600',
}

export function VideoPanel({ sessionId, vehicleId, token, enabled }: Props) {
  const { videoRef, mediaState, connect } = useWebRTC(sessionId, vehicleId, token, enabled)
  const badgeClass = MEDIA_BADGE[mediaState] ?? 'bg-gray-600'

  return (
    <section className="col-span-2 row-span-2 bg-black rounded-lg border border-gray-700 relative overflow-hidden flex items-center justify-center">
      {/* Video stream */}
      <video
        ref={videoRef}
        className="w-full h-full object-contain"
        autoPlay
        playsInline
        muted
      />

      {/* MEDIA STATE badge */}
      <div className="absolute top-2 right-2">
        <span className={`${badgeClass} text-white text-xs font-mono px-2 py-1 rounded`}>
          {mediaState}
        </span>
      </div>

      {/* Overlay when not connected */}
      {mediaState !== 'MEDIA_CONNECTED' && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2">
          {mediaState === 'MEDIA_FAILED' ? (
            <>
              <p className="text-yellow-400 text-sm font-semibold">Video nicht verfügbar</p>
              <p className="text-gray-500 text-xs">Steuerung weiterhin möglich (DEGRADED)</p>
              <button
                onClick={connect}
                className="mt-1 px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-white transition-colors"
              >
                Retry
              </button>
            </>
          ) : mediaState === 'MEDIA_NEGOTIATING' ? (
            <p className="text-blue-400 text-sm">Verbindung wird aufgebaut…</p>
          ) : (
            <p className="text-gray-600 text-sm">
              {enabled ? 'Warte auf Video-Stream…' : 'Verbinden um Video zu starten'}
            </p>
          )}
        </div>
      )}
    </section>
  )
}
