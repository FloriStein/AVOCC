import { useCallback, useEffect, useRef, useState } from 'react'
import { create, toBinary } from '@bufbuild/protobuf'
import { monotonicFactory } from 'ulidx'
import type { WSClient } from '@/lib/ws-client'

// Sprint 3: proto classes generated at build-time (src/gen/).
// Dynamic import avoids TypeScript errors when gen/ doesn't exist locally.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let ControlCommandSchema: any
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let CorrelationHeaderSchema: any
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let CommandType: any

async function loadSchemas() {
  try {
    const ctrl = await import('@/gen/control_pb.js')
    const common = await import('@/gen/common_pb.js')
    ControlCommandSchema = ctrl.ControlCommandSchema
    CommandType = ctrl.CommandType
    CorrelationHeaderSchema = common.CorrelationHeaderSchema
  } catch {
    // gen/ not yet generated — will work after Docker build
  }
}

loadSchemas()

const generateULID = monotonicFactory()

const DEADMAN_INTERVAL_MS = 400

// Dead-man Switch — operator must actively hold (spacebar or button).
// Releasing causes the server-side watchdog to fire SAFE_MODE after 2s (ADR-009).
export function useDeadmanSwitch(
  wsClient: WSClient | null,
  sessionId: string | null,
  vehicleId: string,
  operatorId: string,
  enabled: boolean,
) {
  const [isActive, setIsActive] = useState(false)
  const isActiveRef = useRef(false)

  const sendHold = useCallback(() => {
    if (!wsClient || !sessionId || !ControlCommandSchema) return
    try {
      const header = create(CorrelationHeaderSchema, {
        sessionId,
        eventId: generateULID(),
        vehicleId,
        operatorId,
        timestamp: BigInt(Date.now()),
      })
      const cmd = create(ControlCommandSchema, {
        header,
        type: CommandType.COMMAND_TYPE_DEADMAN_HOLD,
        value: 1.0,
      })
      wsClient.send(toBinary(ControlCommandSchema, cmd))
    } catch {
      // proto not yet generated — fallback to raw ping byte
      wsClient.send(new Uint8Array([0x01]))
    }
  }, [wsClient, sessionId, vehicleId, operatorId])

  const activate = useCallback(() => {
    if (!enabled) return
    isActiveRef.current = true
    setIsActive(true)
  }, [enabled])

  const deactivate = useCallback(() => {
    isActiveRef.current = false
    setIsActive(false)
  }, [])

  // Interval: send DEADMAN_HOLD while held
  useEffect(() => {
    if (!isActive || !enabled) return
    sendHold() // send immediately on activation
    const id = setInterval(sendHold, DEADMAN_INTERVAL_MS)
    return () => clearInterval(id)
  }, [isActive, enabled, sendHold])

  // Keyboard binding: Spacebar
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.code === 'Space' && !e.repeat) { e.preventDefault(); activate() }
    }
    const onKeyUp = (e: KeyboardEvent) => {
      if (e.code === 'Space') { e.preventDefault(); deactivate() }
    }
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
    }
  }, [activate, deactivate])

  // Deactivate when disabled (e.g. SAFE_MODE)
  useEffect(() => {
    if (!enabled) deactivate()
  }, [enabled, deactivate])

  return {
    isActive,
    buttonProps: {
      onMouseDown: activate,
      onMouseUp: deactivate,
      onMouseLeave: deactivate,
    },
  }
}
