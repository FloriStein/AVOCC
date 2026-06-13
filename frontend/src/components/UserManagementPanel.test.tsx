import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import UserManagementPanel from './UserManagementPanel'
import * as apiClient from '@/lib/api-client'

vi.mock('@/lib/api-client', () => ({
  listUsers: vi.fn(),
  createUser: vi.fn(),
  deleteUser: vi.fn(),
  updateUserRole: vi.fn(),
}))

const baseUsers: apiClient.UserInfo[] = [
  { id: 'admin', display_name: 'Administrator', role: 'ADMIN', is_active: true, created_at: '2026-01-01T00:00:00Z' },
  { id: 'op1', display_name: 'Operator One', role: 'OBSERVER', is_active: true, created_at: '2026-01-02T00:00:00Z' },
]

describe('UserManagementPanel', () => {
  const onClose = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(apiClient.listUsers).mockResolvedValue(baseUsers)
    vi.mocked(apiClient.createUser).mockResolvedValue(undefined)
    vi.mocked(apiClient.deleteUser).mockResolvedValue(undefined)
    vi.mocked(apiClient.updateUserRole).mockResolvedValue(undefined)
  })

  it('zeigt alle Nutzer aus der API', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => {
      expect(screen.getByText('Administrator')).toBeInTheDocument()
      expect(screen.getByText('Operator One')).toBeInTheDocument()
    })
  })

  it('kein Löschen-Button für eigenen Account', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Administrator'))
    // Get rows — admin row must not have a delete button
    const rows = screen.getAllByRole('row')
    const adminRow = rows.find(r => within(r).queryByText('admin'))
    expect(adminRow).toBeDefined()
    expect(within(adminRow!).queryByRole('button', { name: /löschen/i })).toBeNull()
  })

  it('Löschen-Button vorhanden für anderen Account', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Operator One'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))
    expect(op1Row).toBeDefined()
    expect(within(op1Row!).getByRole('button', { name: /löschen/i })).toBeInTheDocument()
  })

  it('ruft deleteUser auf nach Bestätigung', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Operator One'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    await userEvent.click(within(op1Row).getByRole('button', { name: /löschen/i }))
    expect(apiClient.deleteUser).toHaveBeenCalledWith('tok', 'op1')
  })

  it('ruft deleteUser NICHT auf wenn Bestätigung abgelehnt', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Operator One'))
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    await userEvent.click(within(op1Row).getByRole('button', { name: /löschen/i }))
    expect(apiClient.deleteUser).not.toHaveBeenCalled()
  })

  it('Anlegen-Button ist disabled wenn Felder leer', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Administrator'))
    expect(screen.getByRole('button', { name: /anlegen/i })).toBeDisabled()
  })

  it('ruft createUser mit korrekten Daten auf', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Administrator'))
    await userEvent.type(screen.getByPlaceholderText('ID'), 'newop')
    await userEvent.type(screen.getByPlaceholderText('Anzeigename'), 'New Op')
    await userEvent.type(screen.getByPlaceholderText('Passwort'), 'secret123')
    await userEvent.click(screen.getByRole('button', { name: /anlegen/i }))
    await waitFor(() =>
      expect(apiClient.createUser).toHaveBeenCalledWith('tok', 'newop', 'New Op', 'secret123', 'OBSERVER')
    )
  })

  it('zeigt API-Fehler als Fehlermeldung an', async () => {
    vi.mocked(apiClient.listUsers).mockRejectedValue(new Error('403 Forbidden'))
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() =>
      expect(screen.getByText(/403 Forbidden/)).toBeInTheDocument()
    )
  })

  it('ruft updateUserRole auf wenn Rolle geändert wird', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Operator One'))
    // Find the role select for op1
    const rows = screen.getAllByRole('row')
    const op1Row = rows.find(r => within(r).queryByText('op1'))!
    const select = within(op1Row).getByRole('combobox')
    await userEvent.selectOptions(select, 'ACTIVE_OPERATOR')
    expect(apiClient.updateUserRole).toHaveBeenCalledWith('tok', 'op1', 'ACTIVE_OPERATOR')
  })

  it('schließt Panel bei Klick auf ✕', async () => {
    render(<UserManagementPanel token="tok" currentUserId="admin" onClose={onClose} />)
    await waitFor(() => screen.getByText('Administrator'))
    await userEvent.click(screen.getByText('✕'))
    expect(onClose).toHaveBeenCalled()
  })
})
