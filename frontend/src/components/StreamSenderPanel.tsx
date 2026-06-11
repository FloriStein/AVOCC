import { useRef, useState } from 'react'
import { useWHIPSender, type SourceType } from '@/hooks/useWHIPSender'

const STATUS_BADGE: Record<string, string> = {
  idle:       'bg-gray-600',
  gathering:  'bg-blue-500 animate-pulse',
  connecting: 'bg-yellow-500 animate-pulse',
  live:       'bg-green-500',
  error:      'bg-red-600',
  stopped:    'bg-gray-600',
}

export function StreamSenderPanel() {
  const previewRef = useRef<HTMLVideoElement>(null)
  const { status, error, candidates, start, stop } = useWHIPSender(previewRef)

  const [streamName, setStreamName] = useState('vehicle-001')
  const [source,     setSource]     = useState<SourceType>('camera')
  const [streamKey,  setStreamKey]  = useState(
    () => localStorage.getItem('avoc-whip-key') ?? ''
  )

  const saveKey = (v: string) => {
    setStreamKey(v)
    localStorage.setItem('avoc-whip-key', v)
  }

  const whipUrl  = `/whip/${streamName}/whip`
  const isLive   = status === 'live'
  const isBusy   = status === 'gathering' || status === 'connecting'

  const srflx = candidates.filter(c => c.type === 'srflx').length
  const relay  = candidates.filter(c => c.type === 'relay').length
  const host   = candidates.filter(c => c.type === 'host').length

  return (
    <div className="bg-gray-800 border-t border-gray-700 px-4 py-3">
      <div className="flex items-start gap-4">

        {/* Vorschau */}
        <div className="relative flex-shrink-0 w-48 h-28 bg-black rounded overflow-hidden border border-gray-700">
          <video ref={previewRef} autoPlay muted playsInline
            className="w-full h-full object-contain" />
          {!isLive && !isBusy && (
            <div className="absolute inset-0 flex items-center justify-center">
              <span className="text-gray-600 text-xs">Vorschau</span>
            </div>
          )}
        </div>

        {/* Controls */}
        <div className="flex-1 flex flex-col gap-2 min-w-0">

          {/* Status + ICE */}
          <div className="flex items-center gap-3 flex-wrap">
            <span className={`${STATUS_BADGE[status] ?? 'bg-gray-600'} text-white text-xs font-mono px-2 py-1 rounded`}>
              {status.toUpperCase()}
            </span>
            {candidates.length > 0 && (
              <span className="text-xs font-mono text-gray-400">
                ICE: {host}h {srflx}s {relay}r
              </span>
            )}
          </div>

          {/* Konfiguration */}
          <div className="flex items-center gap-2 flex-wrap">
            <select
              value={source}
              onChange={e => setSource(e.target.value as SourceType)}
              disabled={isBusy || isLive}
              className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-xs text-white disabled:opacity-50 cursor-pointer"
            >
              <option value="camera">Webcam</option>
              <option value="screen">Bildschirm</option>
            </select>

            <input
              value={streamName}
              onChange={e => setStreamName(e.target.value)}
              disabled={isBusy || isLive}
              placeholder="Stream-Name"
              className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-xs text-white w-32 disabled:opacity-50"
            />

            <input
              type="password"
              value={streamKey}
              onChange={e => saveKey(e.target.value)}
              disabled={isBusy || isLive}
              placeholder="Stream-Key"
              className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-xs text-white w-32 disabled:opacity-50"
            />

            {isLive || isBusy ? (
              <button
                onClick={stop}
                disabled={isBusy}
                className="px-3 py-1 bg-red-700 hover:bg-red-600 disabled:opacity-50 rounded text-xs text-white font-semibold transition-colors"
              >
                Stoppen
              </button>
            ) : (
              <button
                onClick={() => start(whipUrl, source, streamKey)}
                disabled={!streamKey}
                title={!streamKey ? 'Stream-Key erforderlich' : undefined}
                className="px-3 py-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed rounded text-xs text-white font-semibold transition-colors"
              >
                ⏺ Senden
              </button>
            )}
          </div>

          {/* URL + Fehler */}
          <code className="text-gray-500 text-xs truncate">{whipUrl}</code>
          {error && <p className="text-red-400 text-xs">{error}</p>}
        </div>
      </div>
    </div>
  )
}
