import { useRef, useState } from 'react'
import { useControls } from '@/hooks/useControls'
import type { WSClient } from '@/lib/ws-client'

const OPERATOR_ID = 'operator-1'
const JOY_RADIUS = 55
const JOY_TRAVEL = 43

interface Props {
  wsClient: WSClient | null
  sessionId: string | null
  vehicleId: string | null
  enabled: boolean
}

function AxisBar({ value, label }: { value: number; label: string }) {
  const pct = Math.abs(value) * 50
  const isPos = value >= 0
  return (
    <div className="flex flex-col items-center gap-0.5 w-9">
      <span className="text-gray-500 text-xs">{label}</span>
      <div className="h-14 w-3 bg-gray-700 rounded relative overflow-hidden">
        <div className="absolute top-1/2 left-0 right-0 h-px bg-gray-600" />
        <div
          className={`absolute left-0 right-0 ${isPos ? 'bg-green-500' : 'bg-red-500'}`}
          style={{ height: `${pct}%`, ...(isPos ? { bottom: '50%' } : { top: '50%' }) }}
        />
      </div>
      <span className="text-xs font-mono text-gray-500">{value.toFixed(1)}</span>
    </div>
  )
}

const MODE_LABEL: Record<string, string> = {
  none: '—',
  keyboard: 'Keyboard',
  joystick: 'Joystick',
  gamepad: 'Gamepad',
}

const MODE_COLOR: Record<string, string> = {
  none: 'text-gray-600',
  keyboard: 'text-blue-400',
  joystick: 'text-purple-400',
  gamepad: 'text-green-400',
}

export function ControlPanel({ wsClient, sessionId, vehicleId, enabled }: Props) {
  const [speed, setSpeed] = useState(1.0)
  const { steer, throttle, activeMode, gamepadConnected, joyPos, setJoystick } = useControls(
    wsClient, sessionId, vehicleId ?? '', OPERATOR_ID, enabled, speed,
  )

  const isDragging = useRef(false)

  const resolveJoy = (e: React.PointerEvent<SVGSVGElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const cx = rect.left + rect.width / 2
    const cy = rect.top + rect.height / 2
    const scale = rect.width / (JOY_RADIUS * 2)
    let nx = (e.clientX - cx) / (scale * JOY_RADIUS)
    let ny = -(e.clientY - cy) / (scale * JOY_RADIUS)
    const dist = Math.sqrt(nx * nx + ny * ny)
    if (dist > 1) { nx /= dist; ny /= dist }
    setJoystick(nx, ny, true)
  }

  return (
    <div className={`flex items-center gap-5 px-4 py-2 ${!enabled ? 'opacity-40 pointer-events-none select-none' : ''}`}>
      {/* Mode + gamepad indicator */}
      <div className="flex flex-col items-start gap-0.5 min-w-20">
        <span className="text-gray-600 text-xs uppercase tracking-wide">Input</span>
        <span className={`text-sm font-semibold ${MODE_COLOR[activeMode]}`}>
          {MODE_LABEL[activeMode]}
        </span>
        <span className={`text-xs ${gamepadConnected ? 'text-green-500' : 'text-gray-700'}`}>
          Gamepad {gamepadConnected ? '●' : '○'}
        </span>
      </div>

      {/* Steer / Throttle bars */}
      <div className="flex gap-2">
        <AxisBar value={steer} label="Steer" />
        <AxisBar value={throttle} label="Throt" />
      </div>

      {/* Virtual Joystick SVG */}
      <svg
        viewBox={`-${JOY_RADIUS} -${JOY_RADIUS} ${JOY_RADIUS * 2} ${JOY_RADIUS * 2}`}
        className="w-20 h-20 cursor-crosshair touch-none select-none shrink-0"
        onPointerDown={(e) => {
          isDragging.current = true
          e.currentTarget.setPointerCapture(e.pointerId)
          resolveJoy(e)
        }}
        onPointerMove={(e) => { if (isDragging.current) resolveJoy(e) }}
        onPointerUp={() => { isDragging.current = false; setJoystick(0, 0, false) }}
        onPointerCancel={() => { isDragging.current = false; setJoystick(0, 0, false) }}
      >
        <circle cx="0" cy="0" r={JOY_RADIUS - 2} className="fill-gray-700 stroke-gray-600" strokeWidth="2" />
        <line x1={-(JOY_RADIUS - 6)} y1="0" x2={JOY_RADIUS - 6} y2="0" className="stroke-gray-600" strokeWidth="1" />
        <line x1="0" y1={-(JOY_RADIUS - 6)} x2="0" y2={JOY_RADIUS - 6} className="stroke-gray-600" strokeWidth="1" />
        <circle
          cx={joyPos.x * JOY_TRAVEL}
          cy={-joyPos.y * JOY_TRAVEL}
          r="11"
          className={activeMode === 'joystick' ? 'fill-blue-500' : 'fill-gray-500'}
        />
      </svg>

      {/* Speed slider */}
      <div className="flex flex-col items-center gap-1">
        <span className="text-gray-500 text-xs">Speed {Math.round(speed * 100)}%</span>
        <input
          type="range" min="0.1" max="1.0" step="0.05"
          value={speed}
          onChange={(e) => setSpeed(Number(e.target.value))}
          className="w-20 accent-blue-500"
        />
      </div>

      {/* Keyboard hint */}
      <div className="flex flex-col gap-0.5 text-xs text-gray-700 ml-auto">
        <span>WASD / ↑↓←→</span>
        <span>Space = Deadman</span>
      </div>
    </div>
  )
}
