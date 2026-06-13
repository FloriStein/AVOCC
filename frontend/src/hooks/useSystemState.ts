import { useEffect, useRef, useState } from 'react'
import { getState, type SystemStateResponse } from '@/lib/api-client'

const INITIAL: SystemStateResponse = {
  system: 'IDLE',
  control: 'CONTROL_INIT',
  media: 'MEDIA_INIT',
  operator: 'NO_OPERATOR',
}

const POLL_MS = 500
// 3 consecutive failures × 500ms = 1.5s before showing the unreachable banner.
const UNREACHABLE_THRESHOLD = 3

// Polls GET /api/state every 500ms and returns the current 4-layer state.
// unreachable becomes true after UNREACHABLE_THRESHOLD consecutive poll failures.
export function useSystemState() {
  const [state, setState] = useState<SystemStateResponse>(INITIAL)
  const [unreachable, setUnreachable] = useState(false)
  const failCount = useRef(0)

  useEffect(() => {
    let active = true

    const poll = async () => {
      try {
        const s = await getState()
        if (active) {
          setState(s)
          if (failCount.current >= UNREACHABLE_THRESHOLD) setUnreachable(false)
          failCount.current = 0
        }
      } catch {
        failCount.current++
        if (active && failCount.current >= UNREACHABLE_THRESHOLD) setUnreachable(true)
      }
    }

    poll()
    const id = setInterval(poll, POLL_MS)
    return () => {
      active = false
      clearInterval(id)
    }
  }, [])

  return { ...state, unreachable }
}
