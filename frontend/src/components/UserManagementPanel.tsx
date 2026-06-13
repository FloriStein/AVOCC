import { useEffect, useState } from 'react'
import { listUsers, createUser, deleteUser, updateUserRole, UserInfo } from '@/lib/api-client'

interface Props {
  token: string
  currentUserId: string
  onClose: () => void
}

const ROLES = ['OBSERVER', 'STANDBY', 'ACTIVE_OPERATOR', 'ADMIN']

export default function UserManagementPanel({ token, currentUserId, onClose }: Props) {
  const [users, setUsers] = useState<UserInfo[]>([])
  const [error, setError] = useState<string | null>(null)
  const [newId, setNewId] = useState('')
  const [newName, setNewName] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState('OBSERVER')

  const load = async () => {
    try {
      const data = await listUsers(token)
      setUsers(data ?? [])
      setError(null)
    } catch (e) {
      setError(String(e))
    }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!newId || !newName || !newPassword) return
    try {
      await createUser(token, newId, newName, newPassword, newRole)
      setNewId('')
      setNewName('')
      setNewPassword('')
      setNewRole('OBSERVER')
      load()
    } catch (e) {
      setError(String(e))
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm(`Nutzer "${id}" wirklich löschen?`)) return
    try {
      await deleteUser(token, id)
      load()
    } catch (e) {
      setError(String(e))
    }
  }

  const handleRoleChange = async (id: string, role: string) => {
    try {
      await updateUserRole(token, id, role)
      load()
    } catch (e) {
      setError(String(e))
    }
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-2xl rounded-2xl border border-gray-700 bg-gray-900 p-6 shadow-2xl">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-white">Nutzerverwaltung</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-white text-xl leading-none">✕</button>
        </div>

        {error && <p className="mb-3 text-sm text-red-400">{error}</p>}

        <div className="mb-4 overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-700 text-left text-xs text-gray-400">
                <th className="pb-2 pr-3">ID</th>
                <th className="pb-2 pr-3">Anzeigename</th>
                <th className="pb-2 pr-3">Rolle</th>
                <th className="pb-2 pr-3">Aktiv</th>
                <th className="pb-2">Aktion</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b border-gray-800">
                  <td className="py-2 pr-3 font-mono text-gray-300">{u.id}</td>
                  <td className="py-2 pr-3 text-gray-200">{u.display_name}</td>
                  <td className="py-2 pr-3">
                    <select
                      value={u.role}
                      onChange={(e) => handleRoleChange(u.id, e.target.value)}
                      className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-xs text-white"
                    >
                      {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                    </select>
                  </td>
                  <td className="py-2 pr-3">
                    <span className={u.is_active ? 'text-green-400' : 'text-gray-500'}>
                      {u.is_active ? 'Ja' : 'Nein'}
                    </span>
                  </td>
                  <td className="py-2">
                    {u.id !== currentUserId && (
                      <button
                        onClick={() => handleDelete(u.id)}
                        className="rounded px-2 py-1 text-xs text-red-400 hover:bg-red-900/30"
                      >
                        Löschen
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="border-t border-gray-700 pt-4">
          <p className="mb-2 text-xs font-medium text-gray-400">Neuen Nutzer anlegen</p>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <input
              value={newId}
              onChange={(e) => setNewId(e.target.value)}
              placeholder="ID"
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white placeholder-gray-500"
            />
            <input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Anzeigename"
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white placeholder-gray-500"
            />
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="Passwort"
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white placeholder-gray-500"
            />
            <select
              value={newRole}
              onChange={(e) => setNewRole(e.target.value)}
              className="rounded border border-gray-600 bg-gray-800 px-2 py-1 text-sm text-white"
            >
              {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
            </select>
          </div>
          <button
            onClick={handleCreate}
            disabled={!newId || !newName || !newPassword}
            className="mt-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            Anlegen
          </button>
        </div>
      </div>
    </div>
  )
}
