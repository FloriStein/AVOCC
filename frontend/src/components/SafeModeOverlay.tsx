// SAFE MODE Overlay — blocks all control inputs when system is in SAFE_MODE (ADR-009/011).
// Operator must explicitly acknowledge to resume. No auto-resume (ADR-009).

import { FE_OPERATOR_ACK, logEvent } from '@/lib/logger'

interface Props {
  onResume: () => void
  sessionId?: string
}

export function SafeModeOverlay({ onResume, sessionId }: Props) {
  const handleResume = () => {
    logEvent(FE_OPERATOR_ACK, 'Operator Acknowledgment clicked', { sessionId })
    onResume()
  }
  return (
    <div className="fixed inset-0 z-50 flex flex-col items-center justify-center bg-black/90 backdrop-blur-sm">
      <div className="flex flex-col items-center gap-6 max-w-sm text-center">
        <div className="text-red-500 text-6xl font-black tracking-widest">
          SAFE MODE
        </div>

        <div className="text-gray-300 text-sm leading-relaxed">
          System in sicherem Zustand. Fahrzeug steht. Alle Steuerbefehle blockiert.
        </div>

        <div className="w-full h-px bg-red-900" />

        <div className="text-gray-400 text-xs">
          Zur Wiederaufnahme Operator-Bestätigung erforderlich.
        </div>

        <button
          onClick={handleResume}
          className="w-full py-3 px-6 bg-orange-600 hover:bg-orange-500 active:bg-orange-700
                     text-white font-bold rounded-lg text-sm tracking-wide
                     transition-colors focus:outline-none focus:ring-2 focus:ring-orange-400"
        >
          ✓ Resume — Operator Acknowledgment
        </button>
      </div>
    </div>
  )
}
