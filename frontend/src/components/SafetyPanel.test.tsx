import { render, screen } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { SafetyPanel } from './SafetyPanel'

// Mock api-client to prevent real HTTP calls
vi.mock('@/lib/api-client', () => ({
  emergencyStop: vi.fn().mockResolvedValue(undefined),
}))

// Mock useDeadmanSwitch — no WebSocket in unit tests
vi.mock('@/hooks/useDeadmanSwitch', () => ({
  useDeadmanSwitch: vi.fn(() => ({
    isActive: false,
    buttonProps: {
      onMouseDown: vi.fn(),
      onMouseUp: vi.fn(),
      onMouseLeave: vi.fn(),
    },
  })),
}))

describe('SafetyPanel', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('zeigt Emergency Stop Button', () => {
    render(<SafetyPanel systemState="CONNECTED" sessionId="sess-1" wsClient={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).toBeInTheDocument()
  })

  it('Emergency Stop Button ist disabled wenn SAFE_MODE', () => {
    render(<SafetyPanel systemState="SAFE_MODE" sessionId="sess-1" wsClient={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).toBeDisabled()
  })

  it('Emergency Stop Button ist disabled wenn nicht CONNECTED', () => {
    render(<SafetyPanel systemState="IDLE" sessionId={null} wsClient={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).toBeDisabled()
  })

  it('Emergency Stop Button ist aktiv wenn CONNECTED', () => {
    render(<SafetyPanel systemState="CONNECTED" sessionId="sess-1" wsClient={null} />)
    expect(screen.getByRole('button', { name: /emergency stop/i })).not.toBeDisabled()
  })

  it('Dead-man Switch Button ist sichtbar', () => {
    render(<SafetyPanel systemState="CONNECTED" sessionId="sess-1" wsClient={null} />)
    expect(screen.getByText(/Hold|Spacebar|HOLD/i)).toBeInTheDocument()
  })

  it('zeigt DEGRADED-Warnung wenn DEGRADED', () => {
    render(<SafetyPanel systemState="DEGRADED" sessionId="sess-1" wsClient={null} />)
    expect(screen.getByText(/DEGRADED/i)).toBeInTheDocument()
  })
})
