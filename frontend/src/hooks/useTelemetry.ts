import { useEffect, useState } from 'react'

export interface TelemetryData {
  speedKmh: number
  batteryPct: number
  status: string
}

const POLL_MS = 1000

export function useTelemetry(vehicleId: string | null): TelemetryData | null {
  const [data, setData] = useState<TelemetryData | null>(null)

  useEffect(() => {
    if (!vehicleId) { setData(null); return }

    let active = true
    const poll = async () => {
      try {
        const res = await fetch(`/telemetry/latest/${vehicleId}`)
        if (!res.ok) return
        const json = await res.json() as { speed_kmh: number; battery_pct: number; status: string }
        if (active) setData({ speedKmh: json.speed_kmh ?? 0, batteryPct: json.battery_pct ?? 0, status: json.status ?? '' })
      } catch {
        // MQTT data not yet available — keep last known
      }
    }

    poll()
    const id = setInterval(poll, POLL_MS)
    return () => { active = false; clearInterval(id) }
  }, [vehicleId])

  return data
}
