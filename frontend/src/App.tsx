import { useState } from 'react'

type SystemState = 'IDLE' | 'CONNECTING' | 'AUTHENTICATED' | 'CONNECTED' | 'DEGRADED' | 'SAFE_MODE' | 'RECOVERING'

function SystemStateBadge({ state }: { state: SystemState }) {
  const colors: Record<SystemState, string> = {
    IDLE: 'bg-gray-500',
    CONNECTING: 'bg-blue-500',
    AUTHENTICATED: 'bg-blue-400',
    CONNECTED: 'bg-green-500',
    DEGRADED: 'bg-yellow-500',
    SAFE_MODE: 'bg-red-600',
    RECOVERING: 'bg-orange-500',
  }
  return (
    <span className={`px-2 py-1 rounded text-white text-sm font-mono ${colors[state]}`}>
      {state}
    </span>
  )
}

export default function App() {
  const [systemState] = useState<SystemState>('IDLE')

  return (
    <div className="min-h-screen bg-gray-900 text-white flex flex-col">
      {/* Header */}
      <header className="bg-gray-800 border-b border-gray-700 px-6 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold tracking-wide">AVOC — Teleoperation Control Center</h1>
        <SystemStateBadge state={systemState} />
      </header>

      {/* Main Grid */}
      <main className="flex-1 grid grid-cols-3 grid-rows-2 gap-4 p-4">
        {/* Video Panel — spans 2 columns, 2 rows */}
        <section className="col-span-2 row-span-2 bg-black rounded-lg border border-gray-700 flex items-center justify-center">
          <p className="text-gray-500 text-sm">Video Stream (WebRTC)</p>
        </section>

        {/* Safety Panel */}
        <section className="bg-gray-800 rounded-lg border border-red-900 p-4 flex flex-col gap-3">
          <h2 className="text-sm font-semibold text-red-400 uppercase tracking-wide">Safety</h2>
          <button
            className="w-full bg-red-700 hover:bg-red-600 text-white font-bold py-3 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={systemState === 'SAFE_MODE'}
          >
            ⚠ Emergency Stop
          </button>
          <div className="flex items-center gap-2 text-sm">
            <span className="text-gray-400">Dead-man Switch</span>
            <span className="ml-auto text-gray-500">INACTIVE</span>
          </div>
          {systemState === 'SAFE_MODE' && (
            <button className="w-full bg-orange-600 hover:bg-orange-500 text-white font-bold py-2 rounded-lg text-sm">
              Resume (Operator Ack)
            </button>
          )}
        </section>

        {/* Connection Status Panel */}
        <section className="bg-gray-800 rounded-lg border border-gray-700 p-4 flex flex-col gap-2">
          <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wide">Connection</h2>
          <div className="flex justify-between text-sm items-center">
            <span className="text-gray-400">State</span>
            <SystemStateBadge state={systemState} />
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-gray-400">Latency</span>
            <span className="font-mono text-gray-300">— ms</span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-gray-400">Operator</span>
            <span className="text-gray-500 text-xs">—</span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-gray-400">Session ID</span>
            <span className="font-mono text-gray-600 text-xs">—</span>
          </div>
        </section>
      </main>

      {/* Control Bar */}
      <footer className="bg-gray-800 border-t border-gray-700 px-6 py-3 flex items-center gap-4">
        <span className="text-gray-400 text-sm">Control Panel</span>
        <div className="flex gap-2 ml-auto">
          {['Joystick', 'Keyboard', 'Gamepad'].map((ctrl) => (
            <button
              key={ctrl}
              className="px-3 py-1 bg-gray-700 rounded text-sm opacity-50 cursor-not-allowed"
              disabled
            >
              {ctrl}
            </button>
          ))}
        </div>
      </footer>
    </div>
  )
}
