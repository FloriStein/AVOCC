import { render, screen } from '@testing-library/react'
import { vi, describe, it, expect } from 'vitest'
import { ControlPanel } from './ControlPanel'

// useControls hat WebSocket + Gamepad API — im Unit-Test mocken
vi.mock('@/hooks/useControls', () => ({
  useControls: vi.fn(() => ({
    steer: 0,
    throttle: 0,
    activeMode: 'none' as const,
    gamepadConnected: false,
    joyPos: { x: 0, y: 0 },
    setJoystick: vi.fn(),
  })),
}))

describe('ControlPanel', () => {
  it('zeigt Input-Anzeige', () => {
    render(<ControlPanel wsClient={null} sessionId={null} vehicleId={null} enabled={true} />)
    expect(screen.getByText('Input')).toBeInTheDocument()
  })

  it('zeigt Speed-Slider', () => {
    render(<ControlPanel wsClient={null} sessionId={null} vehicleId={null} enabled={true} />)
    expect(screen.getByRole('slider')).toBeInTheDocument()
  })

  it('zeigt Keyboard-Hinweis', () => {
    render(<ControlPanel wsClient={null} sessionId={null} vehicleId={null} enabled={true} />)
    expect(screen.getByText(/WASD/i)).toBeInTheDocument()
  })

  it('zeigt Joystick SVG', () => {
    const { container } = render(<ControlPanel wsClient={null} sessionId={null} vehicleId={null} enabled={true} />)
    expect(container.querySelector('svg')).toBeInTheDocument()
  })

  it('disabled-Zustand setzt opacity-Klasse', () => {
    const { container } = render(<ControlPanel wsClient={null} sessionId={null} vehicleId={null} enabled={false} />)
    const panel = container.firstChild as HTMLElement
    expect(panel.className).toMatch(/opacity-40/)
  })

  it('zeigt Gamepad-Indikator', () => {
    render(<ControlPanel wsClient={null} sessionId={null} vehicleId={null} enabled={true} />)
    expect(screen.getByText(/Gamepad/i)).toBeInTheDocument()
  })
})
