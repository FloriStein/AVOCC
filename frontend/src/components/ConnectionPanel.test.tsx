import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { ConnectionPanel } from './ConnectionPanel'

const baseProps = {
  systemState: 'CONNECTED',
  operatorState: 'ACTIVE_OPERATOR',
  sessionId: '01JTXYZABCDEF1234567890',
  vehicleId: null,
  latency: 0,
}

describe('ConnectionPanel', () => {
  it('zeigt SYSTEM STATE Badge', () => {
    render(<ConnectionPanel {...baseProps} />)
    expect(screen.getByText('CONNECTED')).toBeInTheDocument()
  })

  it('zeigt Operator-Rolle', () => {
    render(<ConnectionPanel {...baseProps} />)
    expect(screen.getByText('ACTIVE_OPERATOR')).toBeInTheDocument()
  })

  it('kürzt Session-ID auf 8 Zeichen + Ellipsis', () => {
    render(<ConnectionPanel {...baseProps} />)
    expect(screen.getByText('01JTXYZA…')).toBeInTheDocument()
  })

  it('zeigt — ms für Control wenn Latenz 0 und — ms für Video wenn kein videoLatency', () => {
    render(<ConnectionPanel {...baseProps} latency={0} />)
    expect(screen.getAllByText('— ms')).toHaveLength(2)
  })

  it('zeigt Latenz in ms wenn > 0', () => {
    render(<ConnectionPanel {...baseProps} latency={42} />)
    expect(screen.getByText('42 ms')).toBeInTheDocument()
  })

  it('Latenz < 50ms → grüne Farbe', () => {
    render(<ConnectionPanel {...baseProps} latency={30} />)
    const el = screen.getByText('30 ms')
    expect(el).toHaveClass('text-green-400')
  })

  it('Latenz 50–99ms → gelbe Farbe', () => {
    render(<ConnectionPanel {...baseProps} latency={75} />)
    const el = screen.getByText('75 ms')
    expect(el).toHaveClass('text-yellow-400')
  })

  it('Latenz ≥ 100ms → rote Farbe', () => {
    render(<ConnectionPanel {...baseProps} latency={120} />)
    const el = screen.getByText('120 ms')
    expect(el).toHaveClass('text-red-400')
  })

  it('zeigt Telemetrie-Daten wenn vorhanden', () => {
    render(<ConnectionPanel
      {...baseProps}
      telemetry={{ speedKmh: 42.5, batteryPct: 85, status: 'MOVING' }}
    />)
    expect(screen.getByText('42.5 km/h')).toBeInTheDocument()
    expect(screen.getByText('85 %')).toBeInTheDocument()
  })

  it('zeigt keine Telemetrie-Zeilen wenn null', () => {
    render(<ConnectionPanel {...baseProps} telemetry={null} />)
    expect(screen.queryByText(/km\/h/)).not.toBeInTheDocument()
  })
})
