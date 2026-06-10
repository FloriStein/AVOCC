import { render, screen } from '@testing-library/react'
import { vi, describe, it, expect } from 'vitest'
import type { MediaState } from '@/hooks/useWebRTC'
import { VideoPanel } from './VideoPanel'

const mockConnect = vi.fn()
const mockDisconnect = vi.fn()

// useWebRTC nutzt RTCPeerConnection — nicht in jsdom verfügbar
const mockUseWebRTC = vi.fn((_sessionId: string | null, _vehicleId: string, _token: string | null, enabled: boolean): {
  videoRef: { current: null }
  mediaState: MediaState
  connect: () => void
  disconnect: () => void
} => ({
  videoRef: { current: null },
  mediaState: (enabled ? 'MEDIA_NEGOTIATING' : 'MEDIA_INIT') as MediaState,
  connect: mockConnect,
  disconnect: mockDisconnect,
}))

vi.mock('@/hooks/useWebRTC', () => ({
  useWebRTC: (...args: Parameters<typeof mockUseWebRTC>) => mockUseWebRTC(...args),
}))

describe('VideoPanel', () => {
  it('zeigt MEDIA_INIT Badge wenn nicht verbunden', () => {
    render(<VideoPanel sessionId={null} vehicleId="vehicle-001" token={null} enabled={false} />)
    expect(screen.getByText('MEDIA_INIT')).toBeInTheDocument()
  })

  it('zeigt MEDIA_NEGOTIATING Badge wenn enabled', () => {
    render(<VideoPanel sessionId="sess-1" vehicleId="vehicle-001" token="test-jwt" enabled={true} />)
    expect(screen.getByText('MEDIA_NEGOTIATING')).toBeInTheDocument()
  })

  it('zeigt MEDIA_FAILED Overlay mit Steuerung-Warnung', () => {
    mockUseWebRTC.mockReturnValueOnce({
      videoRef: { current: null },
      mediaState: 'MEDIA_FAILED',
      connect: mockConnect,
      disconnect: mockDisconnect,
    })
    render(<VideoPanel sessionId="sess-1" vehicleId="vehicle-001" token="test-jwt" enabled={true} />)
    expect(screen.getByText(/Video nicht verfügbar/i)).toBeInTheDocument()
    expect(screen.getByText(/Steuerung weiterhin möglich/i)).toBeInTheDocument()
  })

  it('zeigt Retry-Button bei MEDIA_FAILED', () => {
    mockUseWebRTC.mockReturnValueOnce({
      videoRef: { current: null },
      mediaState: 'MEDIA_FAILED',
      connect: mockConnect,
      disconnect: mockDisconnect,
    })
    render(<VideoPanel sessionId="sess-1" vehicleId="vehicle-001" token="test-jwt" enabled={true} />)
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
  })

  it('zeigt video Element', () => {
    const { container } = render(<VideoPanel sessionId={null} vehicleId="vehicle-001" token={null} enabled={false} />)
    expect(container.querySelector('video')).toBeInTheDocument()
  })
})
