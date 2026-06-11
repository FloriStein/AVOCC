import { useEffect, useState } from 'react'

export interface VehicleAckData {
  commandEventId: string
  received: boolean
  receivedAtMs: number
  vehicleConnected: boolean
  /** ms since last ACK was received, or null if no ACK yet */
  ageSinceAckMs: number | null
}

const POLL_MS = 500

export function useVehicleAck(vehicleId: string | null): VehicleAckData | null {
  const [data, setData] = useState<VehicleAckData | null>(null)

  useEffect(() => {
    if (!vehicleId) { setData(null); return }

    let active = true
    const poll = async () => {
      try {
        const res = await fetch(`/vehicle/ack/latest/${vehicleId}`)
        if (!res.ok) {
          if (res.status === 404) {
            setData(prev => prev ? { ...prev, vehicleConnected: false } : null)
          }
          return
        }
        const json = await res.json() as {
          command_event_id: string
          received: boolean
          received_at_ms: number
          vehicle_connected: boolean
        }
        if (active) {
          setData({
            commandEventId: json.command_event_id ?? '',
            received: json.received ?? false,
            receivedAtMs: json.received_at_ms ?? 0,
            vehicleConnected: json.vehicle_connected ?? false,
            ageSinceAckMs: json.received_at_ms > 0
              ? Date.now() - json.received_at_ms
              : null,
          })
        }
      } catch {
        // control server not reachable — keep last known
      }
    }

    poll()
    const id = setInterval(poll, POLL_MS)
    return () => { active = false; clearInterval(id) }
  }, [vehicleId])

  return data
}
