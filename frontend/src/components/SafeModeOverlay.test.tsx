import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect } from 'vitest'
import { SafeModeOverlay } from './SafeModeOverlay'

describe('SafeModeOverlay', () => {
  it('zeigt SAFE MODE Titel und Beschreibung', () => {
    render(<SafeModeOverlay onResume={vi.fn()} />)
    expect(screen.getByText(/SAFE MODE/i)).toBeInTheDocument()
  })

  it('zeigt Resume-Button', () => {
    render(<SafeModeOverlay onResume={vi.fn()} />)
    expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument()
  })

  it('ruft onResume auf wenn Resume-Button gedrückt wird', async () => {
    const onResume = vi.fn()
    render(<SafeModeOverlay onResume={onResume} />)
    await userEvent.click(screen.getByRole('button', { name: /resume/i }))
    expect(onResume).toHaveBeenCalledOnce()
  })

  it('blockiert UI durch Overlay (z-Index / fullscreen)', () => {
    const { container } = render(<SafeModeOverlay onResume={vi.fn()} />)
    const overlay = container.firstChild as HTMLElement
    expect(overlay).toHaveClass('fixed')
  })
})
