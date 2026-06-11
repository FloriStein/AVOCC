import { render, screen } from '@testing-library/react'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import type { MediaState } from '@/hooks/useWebRTC'
import { VideoPanel } from './VideoPanel'

const mockConnect = vi.fn()
const mockDisconnect = vi.fn()

const mockUseWebRTC = vi.fn((_sessionId: string | null, _vehicleId: string, _token: string | null, enabled: boolean): {
  videoRef: { current: null }
  streamRef: { current: null }
  mediaState: MediaState
  connect: () => void
  disconnect: () => void
} => ({
  videoRef: { current: null },
  streamRef: { current: null },
  mediaState: (enabled ? 'MEDIA_NEGOTIATING' : 'MEDIA_INIT') as MediaState,
  connect: mockConnect,
  disconnect: mockDisconnect,
}))

vi.mock('@/hooks/useWebRTC', () => ({
  useWebRTC: (...args: Parameters<typeof mockUseWebRTC>) => mockUseWebRTC(...args),
}))

const mockStart = vi.fn()
const mockStop = vi.fn()

const mockUseMediaRecorder = vi.fn(() => ({
  isRecording: false,
  duration: '00:00',
  start: mockStart,
  stop: mockStop,
}))

vi.mock('@/hooks/useMediaRecorder', () => ({
  useMediaRecorder: () => mockUseMediaRecorder(),
}))

describe('VideoPanel', () => {
  beforeEach(() => {
    mockUseMediaRecorder.mockReturnValue({ isRecording: false, duration: '00:00', start: mockStart, stop: mockStop })
  })

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
      streamRef: { current: null },
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
      streamRef: { current: null },
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

  it('zeigt REC-Button wenn MEDIA_CONNECTED', () => {
    mockUseWebRTC.mockReturnValueOnce({
      videoRef: { current: null },
      streamRef: { current: null },
      mediaState: 'MEDIA_CONNECTED',
      connect: mockConnect,
      disconnect: mockDisconnect,
    })
    render(<VideoPanel sessionId="sess-1" vehicleId="vehicle-001" token="test-jwt" enabled={true} />)
    expect(screen.getByRole('button', { name: /rec/i })).toBeInTheDocument()
  })

  it('zeigt Stop-Button und Dauer wenn Aufnahme läuft', () => {
    mockUseWebRTC.mockReturnValueOnce({
      videoRef: { current: null },
      streamRef: { current: null },
      mediaState: 'MEDIA_CONNECTED',
      connect: mockConnect,
      disconnect: mockDisconnect,
    })
    mockUseMediaRecorder.mockReturnValueOnce({ isRecording: true, duration: '00:42', start: mockStart, stop: mockStop })
    render(<VideoPanel sessionId="sess-1" vehicleId="vehicle-001" token="test-jwt" enabled={true} />)
    expect(screen.getByText(/00:42/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /stop/i })).toBeInTheDocument()
  })

  it('zeigt keinen REC-Button wenn nicht MEDIA_CONNECTED', () => {
    render(<VideoPanel sessionId="sess-1" vehicleId="vehicle-001" token="test-jwt" enabled={true} />)
    expect(screen.queryByRole('button', { name: /rec/i })).not.toBeInTheDocument()
  })
})
