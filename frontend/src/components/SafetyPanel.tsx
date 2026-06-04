// Safety Panel — Emergency Stop + Dead-man Switch (ADR-009/015).
// Emergency Stop is a CRITICAL trigger — bypasses all layers.
// Dead-man Switch must be actively held; releasing triggers SAFE_MODE after 2s.

import { emergencyStop } from '@/lib/api-client'
import { useDeadmanSwitch } from '@/hooks/useDeadmanSwitch'
import { FE_EMERGENCY_STOP, logEvent } from '@/lib/logger'
import type { WSClient } from '@/lib/ws-client'

interface Props {
  systemState: string
  sessionId: string | null
  wsClient: WSClient | null
}

const VEHICLE_ID = 'vehicle-1'
const OPERATOR_ID = 'operator-1'

export function SafetyPanel({ systemState, sessionId, wsClient }: Props) {
  const isConnected = systemState === 'CONNECTED' || systemState === 'DEGRADED'
  const isSafeMode = systemState === 'SAFE_MODE'

  const deadman = useDeadmanSwitch(
    wsClient,
    sessionId,
    VEHICLE_ID,
    OPERATOR_ID,
    isConnected,
  )

  const handleEmergencyStop = async () => {
    if (isSafeMode) return
    logEvent(FE_EMERGENCY_STOP, 'Emergency Stop clicked',
      { sessionId: sessionId ?? '', vehicleId: VEHICLE_ID, operatorId: OPERATOR_ID })
    try {
      await emergencyStop(sessionId ?? '', VEHICLE_ID)
    } catch {
      // state polling will detect SAFE_MODE regardless
    }
  }

  return (
    <section className="bg-gray-800 rounded-lg border border-red-900 p-4 flex flex-col gap-3">
      <h2 className="text-sm font-semibold text-red-400 uppercase tracking-wide">Safety</h2>

      {/* Emergency Stop */}
      <button
        onClick={handleEmergencyStop}
        disabled={isSafeMode || !isConnected}
        className="w-full bg-red-700 hover:bg-red-600 active:bg-red-800
                   text-white font-bold py-3 rounded-lg
                   disabled:opacity-40 disabled:cursor-not-allowed
                   transition-colors focus:outline-none focus:ring-2 focus:ring-red-400"
      >
        ⚠ Emergency Stop
      </button>

      {/* Dead-man Switch */}
      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between text-sm">
          <span className="text-gray-400">Dead-man Switch</span>
          <span className={`text-xs font-mono px-2 py-0.5 rounded ${
            deadman.isActive
              ? 'bg-green-700 text-green-100'
              : isConnected
                ? 'bg-gray-700 text-gray-400'
                : 'bg-gray-800 text-gray-600'
          }`}>
            {deadman.isActive ? 'ACTIVE' : 'INACTIVE'}
          </span>
        </div>

        <button
          {...deadman.buttonProps}
          disabled={!isConnected}
          className={`w-full py-2 rounded-lg text-sm font-semibold select-none
                      transition-all focus:outline-none
                      ${deadman.isActive
                        ? 'bg-green-700 text-white ring-2 ring-green-400'
                        : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
                      }
                      disabled:opacity-40 disabled:cursor-not-allowed`}
        >
          {deadman.isActive ? '🟢 HOLD' : 'Hold (or Spacebar)'}
        </button>

        {isConnected && !deadman.isActive && (
          <p className="text-xs text-yellow-500 text-center">
            Loslassen → SAFE MODE nach 2s
          </p>
        )}
      </div>

      {/* DEGRADED warning */}
      {systemState === 'DEGRADED' && (
        <div className="text-xs text-yellow-400 bg-yellow-900/30 rounded px-2 py-1 text-center">
          ⚠ DEGRADED — Video/Telemetrie beeinträchtigt
        </div>
      )}
    </section>
  )
}
