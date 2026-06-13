import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import LoginPanel from './LoginPanel'

describe('LoginPanel', () => {
  const onLogin = vi.fn()
  beforeEach(() => vi.clearAllMocks())

  it('rendert ID- und Passwort-Felder sowie Anmelden-Button', () => {
    render(<LoginPanel onLogin={onLogin} />)
    expect(screen.getByPlaceholderText('admin')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('••••••••')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /anmelden/i })).toBeInTheDocument()
  })

  it('Button ist disabled wenn beide Felder leer', () => {
    render(<LoginPanel onLogin={onLogin} />)
    expect(screen.getByRole('button', { name: /anmelden/i })).toBeDisabled()
  })

  it('Button ist disabled wenn nur ID ausgefüllt', async () => {
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('admin'), 'admin')
    expect(screen.getByRole('button', { name: /anmelden/i })).toBeDisabled()
  })

  it('Button ist disabled wenn nur Passwort ausgefüllt', async () => {
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'secret')
    expect(screen.getByRole('button', { name: /anmelden/i })).toBeDisabled()
  })

  it('Button ist enabled wenn beide Felder ausgefüllt', async () => {
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('admin'), 'admin')
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'secret')
    expect(screen.getByRole('button', { name: /anmelden/i })).toBeEnabled()
  })

  it('ruft onLogin mit ID und Passwort auf bei Submit', async () => {
    onLogin.mockResolvedValue(undefined)
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('admin'), 'admin')
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'mypassword')
    fireEvent.click(screen.getByRole('button', { name: /anmelden/i }))
    await waitFor(() => expect(onLogin).toHaveBeenCalledWith('admin', 'mypassword'))
  })

  it('zeigt Fehlermeldung wenn onLogin wirft', async () => {
    onLogin.mockRejectedValue(new Error('401'))
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('admin'), 'admin')
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'wrong')
    fireEvent.click(screen.getByRole('button', { name: /anmelden/i }))
    await waitFor(() =>
      expect(screen.getByText('Ungültige Zugangsdaten')).toBeInTheDocument()
    )
  })

  it('zeigt während Login "Anmelden…" und deaktiviert Button', async () => {
    let resolve!: () => void
    onLogin.mockReturnValue(new Promise<void>((r) => { resolve = r }))
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('admin'), 'admin')
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'pass')
    fireEvent.click(screen.getByRole('button', { name: /anmelden/i }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /anmelden…/i })).toBeDisabled()
    )
    resolve()
  })

  it('löscht Fehlermeldung nicht automatisch bei erneutem Versuch — neuer Submit ersetzt sie', async () => {
    onLogin.mockRejectedValueOnce(new Error('401')).mockResolvedValue(undefined)
    render(<LoginPanel onLogin={onLogin} />)
    await userEvent.type(screen.getByPlaceholderText('admin'), 'admin')
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'wrong')
    fireEvent.click(screen.getByRole('button', { name: /anmelden/i }))
    await waitFor(() => expect(screen.getByText('Ungültige Zugangsdaten')).toBeInTheDocument())
    // Second attempt — error should clear while pending
    fireEvent.click(screen.getByRole('button', { name: /anmelden/i }))
    await waitFor(() =>
      expect(screen.queryByText('Ungültige Zugangsdaten')).not.toBeInTheDocument()
    )
  })
})
