import { useEffect } from 'react'
import { useSystemState } from '@/hooks/useSystemState'
import { useSession } from '@/hooks/useSession'
import { useTelemetry } from '@/hooks/useTelemetry'
import { SafeModeOverlay } from '@/components/SafeModeOverlay'
import { SafetyPanel } from '@/components/SafetyPanel'
import { ConnectionPanel } from '@/components/ConnectionPanel'
import { VideoPanel } from '@/components/VideoPanel'
import { ControlPanel } from '@/components/ControlPanel'

const VEHICLE_ID = 'vehicle-1'

const STATE_COLORS: Record<string, string> = {
  IDLE:          'bg-gray-500',
  CONNECTING:    'bg-blue-500',
  AUTHENTICATED: 'bg-blue-400',
  CONNECTED:     'bg-green-500',
  DEGRADED:      'bg-yellow-500',
  SAFE_MODE:     'bg-red-600',
  RECOVERING:    'bg-orange-500',
}

function SystemStateBadge({ state }: { state: string }) {
  const color = STATE_COLORS[state] ?? 'bg-gray-600'
  return (
    <span className={`px-2 py-1 rounded text-white text-sm font-mono ${color}`}>
      {state}
    </span>
  )
}

const OPERATOR_ROLE_LABEL: Record<string, string> = {
  NO_OPERATOR:       '',
  OPERATOR_ASSIGNED: 'Assigned',
  ACTIVE_OPERATOR:   'Active Operator',
  HANDOVER_PENDING:  'Handover…',
  RECOVERING_OPERATOR: 'Recovering',
}

export default function App() {
  const state = useSystemState()
  const session = useSession()
  const telemetry = useTelemetry(session.sessionId ? VEHICLE_ID : null)

  const isConnected = state.system === 'CONNECTED' || state.system === 'DEGRADED'
  const isSafeMode = state.system === 'SAFE_MODE'

  // Auto-connect on app load
  useEffect(() => {
    session.connect()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // When AUTHENTICATED → start session (AUTHENTICATED → CONNECTED)
  useEffect(() => {
    if (state.system === 'AUTHENTICATED') {
      session.startSessionIfNeeded()
    }
  }, [state.system, session])

  return (
    <div className="min-h-screen bg-gray-900 text-white flex flex-col">
      {/* SAFE MODE overlay — blocks everything when system is in SAFE_MODE */}
      {isSafeMode && <SafeModeOverlay onResume={session.resume} />}

      {/* Header */}
      <header className="bg-gray-800 border-b border-gray-700 px-6 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold tracking-wide">AVOC — Teleoperation Control Center</h1>
        <div className="flex items-center gap-3">
          {/* Operator role badge */}
          {state.operator !== 'NO_OPERATOR' && (
            <span className="text-xs font-mono text-gray-400 bg-gray-700 px-2 py-1 rounded">
              {OPERATOR_ROLE_LABEL[state.operator] ?? state.operator}
            </span>
          )}
          {state.system === 'IDLE' && (
            <button
              onClick={session.connect}
              className="px-3 py-1 bg-blue-600 hover:bg-blue-500 rounded text-sm font-semibold"
            >
              Connect
            </button>
          )}
          <SystemStateBadge state={state.system} />
        </div>
      </header>

      {/* DEGRADED banner */}
      {state.system === 'DEGRADED' && (
        <div className="bg-yellow-900/50 border-b border-yellow-700 px-6 py-2 text-yellow-300 text-sm text-center">
          ⚠ DEGRADED — Video oder Telemetrie ausgefallen. Steuerung weiterhin möglich.
        </div>
      )}

      {/* Main Grid */}
      <main className="flex-1 grid grid-cols-3 grid-rows-2 gap-4 p-4 min-h-0">
        {/* Video Panel — 2 columns, 2 rows */}
        <VideoPanel sessionId={session.sessionId} enabled={isConnected} />

        {/* Safety Panel */}
        <SafetyPanel
          systemState={state.system}
          sessionId={session.sessionId}
          wsClient={session.wsClient}
        />

        {/* Connection + Telemetry Panel */}
        <ConnectionPanel
          systemState={state.system}
          operatorState={state.operator}
          sessionId={session.sessionId}
          latency={session.latency}
          telemetry={telemetry}
        />
      </main>

      {/* Control Panel Footer */}
      <footer className="bg-gray-800 border-t border-gray-700">
        <ControlPanel
          wsClient={session.wsClient}
          sessionId={session.sessionId}
          enabled={isConnected}
        />
      </footer>
    </div>
  )
}
