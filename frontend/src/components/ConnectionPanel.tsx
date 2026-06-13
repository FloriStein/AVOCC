// Connection Status Panel — live SYSTEM STATE, latency, session-ID, operator role, telemetry (ADR-016).

import { useState } from 'react'
import { VehicleSelector } from '@/components/VehicleSelector'

interface TelemetryData {
  speedKmh: number
  batteryPct: number
  status: string
}

interface Props {
  systemState: string
  operatorState: string
  sessionId: string | null
  vehicleId: string | null
  latency: number
  telemetry?: TelemetryData | null
  onStartSession?: (vehicleId: string) => void
  onEndSession?: () => Promise<void>
}

const STATE_COLORS: Record<string, string> = {
  IDLE:          'bg-gray-500',
  CONNECTING:    'bg-blue-500',
  AUTHENTICATED: 'bg-blue-400',
  CONNECTED:     'bg-green-500',
  DEGRADED:      'bg-yellow-500',
  SAFE_MODE:     'bg-red-600',
  RECOVERING:    'bg-orange-500',
}

function StateBadge({ state }: { state: string }) {
  const color = STATE_COLORS[state] ?? 'bg-gray-600'
  return (
    <span className={`px-2 py-0.5 rounded text-white text-xs font-mono ${color}`}>
      {state}
    </span>
  )
}

function LatencyColor(ms: number): string {
  if (ms === 0) return 'text-gray-500'
  if (ms < 50) return 'text-green-400'
  if (ms < 100) return 'text-yellow-400'
  return 'text-red-400'
}

export function ConnectionPanel({ systemState, operatorState, sessionId, vehicleId, latency, telemetry, onStartSession, onEndSession }: Props) {
  const shortId = sessionId ? sessionId.slice(0, 8) + '…' : '—'
  const [ending, setEnding] = useState(false)

  const isActive = systemState === 'CONNECTED' || systemState === 'DEGRADED'

  const handleEndSession = async () => {
    if (!onEndSession) return
    setEnding(true)
    try {
      await onEndSession()
    } finally {
      setEnding(false)
    }
  }

  return (
    <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-2">
      <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">Connection</h2>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">State</span>
        <StateBadge state={systemState} />
      </div>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Latency</span>
        <span className={`font-mono text-sm ${LatencyColor(latency)}`}>
          {latency > 0 ? `${latency} ms` : '— ms'}
        </span>
      </div>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Operator</span>
        <span className="text-gray-300 text-xs font-mono">{operatorState}</span>
      </div>

      <div className="flex justify-between text-sm items-center">
        <span className="text-gray-400">Session</span>
        <span className="font-mono text-gray-500 text-xs" title={sessionId ?? ''}>
          {shortId}
        </span>
      </div>

      {/* Active session — shown when CONNECTED or DEGRADED */}
      {isActive && (
        <div className="mt-1 flex flex-col gap-2 border-t border-gray-700 pt-2">
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">Fahrzeug</span>
            <span className="font-mono text-green-400 text-xs">{vehicleId ?? '—'}</span>
          </div>
          <button
            onClick={handleEndSession}
            disabled={ending}
            className="w-full py-1.5 px-3 bg-gray-700 hover:bg-red-900 border border-gray-600 hover:border-red-700
                       disabled:opacity-50 disabled:cursor-not-allowed rounded text-xs text-gray-300
                       hover:text-red-300 font-semibold transition-colors"
          >
            {ending ? 'Beende…' : '⏹ Session beenden'}
          </button>
        </div>
      )}

      {/* Vehicle selector — shown when waiting for session start */}
      {systemState === 'AUTHENTICATED' && onStartSession && (
        <VehicleSelector
          onStartSession={onStartSession}
          defaultVehicleId={vehicleId}
        />
      )}

      {/* Reconnecting hint */}
      {systemState === 'RECOVERING' && (
        <p className="text-xs text-orange-400 text-center mt-1">Verbinde neu…</p>
      )}

      {/* Telemetry — shown when MQTT data is available */}
      {telemetry && (
        <>
          <hr className="border-gray-700" />
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">Speed</span>
            <span className="font-mono text-blue-300 text-sm">{telemetry.speedKmh.toFixed(1)} km/h</span>
          </div>
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">Battery</span>
            <span className={`font-mono text-sm ${telemetry.batteryPct < 20 ? 'text-red-400' : 'text-green-400'}`}>
              {telemetry.batteryPct.toFixed(0)} %
            </span>
          </div>
          {telemetry.status && (
            <div className="flex justify-between text-sm items-center">
              <span className="text-gray-400">Status</span>
              <span className="text-gray-300 text-xs font-mono">{telemetry.status}</span>
            </div>
          )}
        </>
      )}
    </section>
  )
}
