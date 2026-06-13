import { useEffect, useState } from 'react'
import { listVehicles, type VehicleInfo } from '@/lib/api-client'

export function useVehicles() {
  const [vehicles, setVehicles] = useState<VehicleInfo[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true

    const poll = async () => {
      try {
        const data = await listVehicles()
        if (active) setVehicles(data)
      } catch {
        // keep stale list on error
      } finally {
        if (active) setLoading(false)
      }
    }

    poll()
    const tid = setInterval(poll, 2000)
    return () => { active = false; clearInterval(tid) }
  }, [])

  return { vehicles, loading }
}
