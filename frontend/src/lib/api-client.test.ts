import { describe, it, expect } from 'vitest'
import { parseTokenRole } from './api-client'

// Build a minimal JWT payload (base64url encoded) — no signature needed for parseTokenRole
function makeToken(payload: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
  const body = btoa(JSON.stringify(payload))
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
  return `${header}.${body}.fakesignature`
}

describe('parseTokenRole', () => {
  it('gibt die Rolle aus einem validen JWT zurück', () => {
    const token = makeToken({ sub: 'admin', role: 'ADMIN' })
    expect(parseTokenRole(token)).toBe('ADMIN')
  })

  it('gibt OBSERVER zurück wenn role=OBSERVER', () => {
    const token = makeToken({ sub: 'op1', role: 'OBSERVER' })
    expect(parseTokenRole(token)).toBe('OBSERVER')
  })

  it('gibt leeren String zurück wenn role fehlt', () => {
    const token = makeToken({ sub: 'op1' })
    expect(parseTokenRole(token)).toBe('')
  })

  it('gibt leeren String zurück bei komplett ungültigem Token', () => {
    expect(parseTokenRole('not.a.token')).toBe('')
  })

  it('gibt leeren String zurück bei leerem String', () => {
    expect(parseTokenRole('')).toBe('')
  })

  it('gibt leeren String zurück wenn Token nur einen Teil hat', () => {
    expect(parseTokenRole('onlyone')).toBe('')
  })

  it('gibt leeren String zurück bei ungültigem base64 im Payload', () => {
    expect(parseTokenRole('header.!!!.sig')).toBe('')
  })

  it('gibt leeren String zurück wenn Payload kein JSON ist', () => {
    const badPayload = btoa('this is not json').replace(/=/g, '')
    expect(parseTokenRole(`header.${badPayload}.sig`)).toBe('')
  })
})
