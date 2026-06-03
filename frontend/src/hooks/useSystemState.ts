import { useEffect, useState } from 'react'
import { getState, type SystemStateResponse } from '@/lib/api-client'

const INITIAL: SystemStateResponse = {
  system: 'IDLE',
  control: 'CONTROL_INIT',
  media: 'MEDIA_INIT',
  operator: 'NO_OPERATOR',
}

const POLL_MS = 500

// Polls GET /api/state every 500ms and returns the current 4-layer state.
// Polling stops when the component unmounts.
export function useSystemState() {
  const [state, setState] = useState<SystemStateResponse>(INITIAL)

  useEffect(() => {
    let active = true

    const poll = async () => {
      try {
        const s = await getState()
        if (active) setState(s)
      } catch {
        // backend unreachable — keep last known state
      }
    }

    poll()
    const id = setInterval(poll, POLL_MS)
    return () => {
      active = false
      clearInterval(id)
    }
  }, [])

  return state
}
