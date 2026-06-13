import { useState } from 'react'
import { useVehicles } from '@/hooks/useVehicles'

interface Props {
  onStartSession: (vehicleId: string) => void
  defaultVehicleId?: string | null
}

export function VehicleSelector({ onStartSession, defaultVehicleId }: Props) {
  const { vehicles, loading } = useVehicles()
  const [selected, setSelected] = useState(defaultVehicleId ?? '')

  return (
    <div className="flex items-center gap-2 mt-2">
      <select
        value={selected}
        onChange={e => setSelected(e.target.value)}
        disabled={loading}
        className="bg-gray-900 border border-gray-600 rounded px-2 py-1 text-xs text-white disabled:opacity-50 cursor-pointer"
      >
        <option value="">
          {loading ? 'Lädt…' : vehicles.length === 0 ? 'Keine Fahrzeuge' : '— Fahrzeug wählen —'}
        </option>
        {vehicles.map(v => (
          <option key={v.id} value={v.id}>
            {v.online ? '🟢' : '⚪'} {v.display_name}
          </option>
        ))}
      </select>

      <button
        onClick={() => selected && onStartSession(selected)}
        disabled={!selected}
        className="px-3 py-1 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed rounded text-xs text-white font-semibold transition-colors"
      >
        Session starten
      </button>
    </div>
  )
}
