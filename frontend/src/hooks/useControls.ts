import { useCallback, useEffect, useRef, useState } from 'react'
import { create, toBinary } from '@bufbuild/protobuf'
import { monotonicFactory } from 'ulidx'
import type { WSClient } from '@/lib/ws-client'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let ControlCommandSchema: any
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let CorrelationHeaderSchema: any
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let CommandType: any
let schemasLoaded = false

async function loadSchemas() {
  if (schemasLoaded) return
  try {
    const ctrl = await import('@/gen/control_pb.js')
    const common = await import('@/gen/common_pb.js')
    ControlCommandSchema = ctrl.ControlCommandSchema
    CommandType = ctrl.CommandType
    CorrelationHeaderSchema = common.CorrelationHeaderSchema
    schemasLoaded = true
  } catch {
    // gen/ not yet generated — works after Docker build
  }
}

loadSchemas()

const generateULID = monotonicFactory()
const INTERVAL_MS = 50 // 20 Hz
const MOVE_KEYS = new Set([
  'KeyW', 'KeyA', 'KeyS', 'KeyD',
  'ArrowUp', 'ArrowLeft', 'ArrowDown', 'ArrowRight',
])

export type ControlMode = 'none' | 'keyboard' | 'joystick' | 'gamepad'

export function useControls(
  wsClient: WSClient | null,
  sessionId: string | null,
  vehicleId: string,
  operatorId: string,
  enabled: boolean,
  speedMultiplier: number,
) {
  const [steer, setSteer] = useState(0)
  const [throttle, setThrottle] = useState(0)
  const [activeMode, setActiveMode] = useState<ControlMode>('none')
  const [gamepadConnected, setGamepadConnected] = useState(false)
  const [joyPos, setJoyPos] = useState({ x: 0, y: 0 })

  const heldKeys = useRef(new Set<string>())
  const joyActiveRef = useRef(false)
  const joyPosRef = useRef({ x: 0, y: 0 })
  const speedRef = useRef(speedMultiplier)

  useEffect(() => { speedRef.current = speedMultiplier }, [speedMultiplier])

  const sendCmd = useCallback((type: number, value: number) => {
    if (!wsClient || !sessionId || !enabled) return
    if (!ControlCommandSchema) {
      wsClient.send(new Uint8Array([0x01]))
      return
    }
    try {
      const header = create(CorrelationHeaderSchema, {
        sessionId, eventId: generateULID(), vehicleId, operatorId,
        timestamp: BigInt(Date.now()),
      })
      wsClient.send(toBinary(ControlCommandSchema, create(ControlCommandSchema, { header, type, value })))
    } catch {
      wsClient.send(new Uint8Array([0x01]))
    }
  }, [wsClient, sessionId, vehicleId, operatorId, enabled])

  // 20 Hz command loop
  useEffect(() => {
    if (!enabled) {
      setSteer(0); setThrottle(0); setActiveMode('none')
      return
    }
    const id = setInterval(() => {
      const s = speedRef.current

      // Priority 1: Gamepad
      const gp = Array.from(navigator.getGamepads()).find(g => g?.connected) ?? null
      if (gp) {
        setGamepadConnected(true)
        const sv = Math.abs(gp.axes[0]) > 0.1 ? gp.axes[0] * s : 0
        const tv = Math.abs(gp.axes[1]) > 0.1 ? -gp.axes[1] * s : 0
        const bv = gp.buttons[6]?.value ?? 0
        if (sv !== 0 || tv !== 0 || bv > 0.1) {
          if (sv !== 0) sendCmd(CommandType?.STEER ?? 1, sv)
          if (tv !== 0) sendCmd(CommandType?.THROTTLE ?? 2, tv)
          if (bv > 0.1) sendCmd(CommandType?.BRAKE ?? 3, bv)
          setSteer(sv); setThrottle(tv); setActiveMode('gamepad')
        } else {
          setSteer(0); setThrottle(0); setActiveMode('none')
        }
        return
      }
      setGamepadConnected(false)

      // Priority 2: Virtual Joystick
      if (joyActiveRef.current) {
        const { x, y } = joyPosRef.current
        const sv = x * s
        const tv = y * s
        if (Math.abs(sv) > 0.01) sendCmd(CommandType?.STEER ?? 1, sv)
        if (Math.abs(tv) > 0.01) sendCmd(CommandType?.THROTTLE ?? 2, tv)
        setSteer(sv); setThrottle(tv); setActiveMode('joystick')
        return
      }

      // Priority 3: Keyboard
      const keys = heldKeys.current
      let sv = 0, tv = 0
      if (keys.has('KeyA') || keys.has('ArrowLeft')) sv -= 1
      if (keys.has('KeyD') || keys.has('ArrowRight')) sv += 1
      if (keys.has('KeyW') || keys.has('ArrowUp')) tv += 1
      if (keys.has('KeyS') || keys.has('ArrowDown')) tv -= 1

      if (sv !== 0 || tv !== 0) {
        if (sv !== 0) sendCmd(CommandType?.STEER ?? 1, sv * s)
        if (tv !== 0) sendCmd(CommandType?.THROTTLE ?? 2, tv * s)
        setSteer(sv * s); setThrottle(tv * s); setActiveMode('keyboard')
      } else {
        setSteer(0); setThrottle(0); setActiveMode('none')
      }
    }, INTERVAL_MS)
    return () => clearInterval(id)
  }, [enabled, sendCmd])

  // Keyboard event listeners
  useEffect(() => {
    const onDown = (e: KeyboardEvent) => {
      if (MOVE_KEYS.has(e.code)) { e.preventDefault(); heldKeys.current.add(e.code) }
    }
    const onUp = (e: KeyboardEvent) => { heldKeys.current.delete(e.code) }
    window.addEventListener('keydown', onDown)
    window.addEventListener('keyup', onUp)
    return () => {
      window.removeEventListener('keydown', onDown)
      window.removeEventListener('keyup', onUp)
    }
  }, [])

  // Clear state when disabled (e.g. SAFE_MODE)
  useEffect(() => {
    if (!enabled) {
      heldKeys.current.clear()
      joyActiveRef.current = false
      joyPosRef.current = { x: 0, y: 0 }
      setJoyPos({ x: 0, y: 0 })
    }
  }, [enabled])

  const setJoystick = useCallback((x: number, y: number, active: boolean) => {
    joyActiveRef.current = active
    joyPosRef.current = { x, y }
    setJoyPos({ x, y })
  }, [])

  return { steer, throttle, activeMode, gamepadConnected, joyPos, setJoystick }
}
