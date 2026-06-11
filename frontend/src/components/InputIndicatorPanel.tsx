import type { TelemetryData } from '@/hooks/useTelemetry'
import type { VehicleAckData } from '@/hooks/useVehicleAck'

interface Props {
  telemetry: TelemetryData | null
  ack: VehicleAckData | null
}

// Rotating steering wheel SVG — angle in degrees (negative = left, positive = right)
function SteeringWheel({ angle }: { angle: number }) {
  return (
    <svg viewBox="-32 -32 64 64" className="w-16 h-16 shrink-0">
      <g transform={`rotate(${angle})`}>
        {/* Outer ring */}
        <circle cx="0" cy="0" r="28" fill="none" stroke="#4b5563" strokeWidth="5" />
        {/* Hub */}
        <circle cx="0" cy="0" r="6" fill="#6b7280" />
        {/* 3 spokes */}
        <line x1="0" y1="-6" x2="0" y2="-22" stroke="#6b7280" strokeWidth="3" strokeLinecap="round" />
        <line x1="6" y1="3" x2="19" y2="10" stroke="#6b7280" strokeWidth="3" strokeLinecap="round" />
        <line x1="-6" y1="3" x2="-19" y2="10" stroke="#6b7280" strokeWidth="3" strokeLinecap="round" />
        {/* Top marker */}
        <circle cx="0" cy="-22" r="3" fill="#3b82f6" />
      </g>
    </svg>
  )
}

// Vertical bar: value [-1, 1] for throttle/steer, [0, 1] for brake
function ActuationBar({
  commanded,
  actual,
  label,
  color = 'green',
}: {
  commanded: number
  actual: number
  label: string
  color?: 'green' | 'red' | 'blue'
}) {
  const colorMap = { green: 'bg-green-500', red: 'bg-red-500', blue: 'bg-blue-500' }
  const cmdPct = Math.abs(commanded) * 50
  const actPct = Math.abs(actual) * 50
  const isPos = commanded >= 0

  return (
    <div className="flex flex-col items-center gap-0.5 w-10">
      <span className="text-gray-500 text-xs">{label}</span>
      {/* Commanded bar */}
      <div className="h-16 w-4 bg-gray-700 rounded relative overflow-hidden" title={`Soll: ${commanded.toFixed(2)}`}>
        <div className="absolute top-1/2 left-0 right-0 h-px bg-gray-600" />
        <div
          className={`absolute left-0 right-0 opacity-40 ${colorMap[color]}`}
          style={{ height: `${cmdPct}%`, ...(isPos ? { bottom: '50%' } : { top: '50%' }) }}
        />
        {/* Actual bar */}
        <div
          className={`absolute left-0 right-0 ${colorMap[color]}`}
          style={{ height: `${actPct}%`, ...(actual >= 0 ? { bottom: '50%' } : { top: '50%' }) }}
        />
      </div>
      <span className="text-xs font-mono text-gray-400">{actual.toFixed(2)}</span>
    </div>
  )
}

function AckBadge({ ack }: { ack: VehicleAckData | null }) {
  if (!ack) {
    return (
      <div className="flex flex-col gap-0.5">
        <span className="text-gray-600 text-xs uppercase tracking-wide">Fahrzeug</span>
        <span className="text-xs text-gray-600">Nicht verbunden</span>
      </div>
    )
  }

  const connected = ack.vehicleConnected
  const age = ack.ageSinceAckMs

  return (
    <div className="flex flex-col gap-1 min-w-24">
      <span className="text-gray-500 text-xs uppercase tracking-wide">Fahrzeug</span>
      <div className="flex items-center gap-1.5">
        <span className={`w-2 h-2 rounded-full shrink-0 ${connected ? 'bg-green-500' : 'bg-red-500'}`} />
        <span className={`text-xs font-semibold ${connected ? 'text-green-400' : 'text-red-400'}`}>
          {connected ? 'Verbunden' : 'Getrennt'}
        </span>
      </div>
      {ack.received && age !== null && (
        <span className="text-xs text-gray-500">
          ACK vor {age < 1000 ? `${age}ms` : `${(age / 1000).toFixed(1)}s`}
        </span>
      )}
      {!ack.received && (
        <span className="text-xs text-yellow-600">Kein ACK</span>
      )}
    </div>
  )
}

export function InputIndicatorPanel({ telemetry, ack }: Props) {
  const steerAngle = (telemetry?.steerActual ?? 0) * 120  // max ±120°
  const steerCommanded = telemetry?.steerCommanded ?? 0
  const steerActual = telemetry?.steerActual ?? 0
  const throttleCommanded = telemetry?.throttleCommanded ?? 0
  const throttleActual = telemetry?.throttleActual ?? 0
  const brakeCommanded = telemetry?.brakeCommanded ?? 0

  const hasData = telemetry !== null &&
    (Math.abs(steerActual) > 0.01 || Math.abs(throttleActual) > 0.01)

  return (
    <div className="flex items-center gap-5 px-4 py-2 border-t border-gray-700">
      {/* Section label */}
      <div className="flex flex-col items-start gap-0.5 min-w-16">
        <span className="text-gray-600 text-xs uppercase tracking-wide">Fahrzeug</span>
        <span className="text-xs text-gray-600">Ist-Werte</span>
        {!hasData && (
          <span className="text-xs text-gray-700">—</span>
        )}
      </div>

      {/* Steering wheel */}
      <div className="flex flex-col items-center gap-0.5">
        <SteeringWheel angle={steerAngle} />
        <span className="text-xs font-mono text-gray-500">{(steerActual * 100).toFixed(0)}%</span>
      </div>

      {/* Actuation bars: Steer / Throttle / Brake */}
      <div className="flex gap-1.5">
        <ActuationBar
          commanded={steerCommanded}
          actual={steerActual}
          label="Steer"
          color="blue"
        />
        <ActuationBar
          commanded={throttleCommanded}
          actual={throttleActual}
          label="Throt"
          color="green"
        />
        <ActuationBar
          commanded={brakeCommanded}
          actual={brakeCommanded}
          label="Brake"
          color="red"
        />
      </div>

      {/* Speed */}
      <div className="flex flex-col items-center gap-0.5 min-w-12">
        <span className="text-gray-500 text-xs">km/h</span>
        <span className="text-xl font-mono font-bold text-white">
          {(telemetry?.speedKmh ?? 0).toFixed(1)}
        </span>
      </div>

      {/* ACK status */}
      <div className="ml-auto">
        <AckBadge ack={ack} />
      </div>
    </div>
  )
}
