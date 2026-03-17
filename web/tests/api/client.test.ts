import { describe, it, expect, beforeEach, vi } from 'vitest'
import {
  request,
  AuthError,
  ApiError,
  getAdminKey,
  setAdminKey,
  clearAdminKey,
} from '@/api/client'

const STORAGE_KEY = 'kestrel_auth'

describe('admin key storage', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns null when no key stored', () => {
    expect(getAdminKey()).toBeNull()
  })

  it('stores and retrieves admin key', () => {
    setAdminKey('test-key-123')
    expect(getAdminKey()).toBe('test-key-123')
  })

  it('clears admin key', () => {
    setAdminKey('test-key-123')
    clearAdminKey()
    expect(getAdminKey()).toBeNull()
  })

  it('returns null for invalid JSON in storage', () => {
    localStorage.setItem(STORAGE_KEY, 'not-json')
    expect(getAdminKey()).toBeNull()
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('returns null for wrong version', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ version: 999, adminKey: 'old' }),
    )
    expect(getAdminKey()).toBeNull()
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
  })
})

describe('request', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it('sends X-Admin-Key header when key is set', async () => {
    setAdminKey('my-admin-key')

    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await request('/admin/accounts')

    expect(fetch).toHaveBeenCalledWith('/admin/accounts', {
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Key': 'my-admin-key',
      },
    })
  })

  it('does not send X-Admin-Key when no key is set', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    await request('/health')

    expect(fetch).toHaveBeenCalledWith('/health', {
      headers: {
        'Content-Type': 'application/json',
      },
    })
  })

  it('returns parsed JSON on success', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ data: [1, 2, 3] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const result = await request<{ data: number[] }>('/admin/accounts')
    expect(result).toEqual({ data: [1, 2, 3] })
  })

  it('dispatches kestrel:auth_error and throws AuthError on 401', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('', { status: 401 }),
    )

    const handler = vi.fn()
    window.addEventListener('kestrel:auth_error', handler)

    await expect(request('/admin/accounts')).rejects.toThrow(AuthError)
    expect(handler).toHaveBeenCalledTimes(1)

    window.removeEventListener('kestrel:auth_error', handler)
  })

  it('dispatches kestrel:auth_error and throws AuthError on 403', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('', { status: 403 }),
    )

    const handler = vi.fn()
    window.addEventListener('kestrel:auth_error', handler)

    await expect(request('/admin/accounts')).rejects.toThrow(AuthError)
    expect(handler).toHaveBeenCalledTimes(1)

    window.removeEventListener('kestrel:auth_error', handler)
  })

  it('throws ApiError with message from error body', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({ error: { message: 'Not found' } }),
        { status: 404, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    try {
      await request('/admin/accounts/xyz')
      expect.unreachable('should have thrown')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      expect((err as ApiError).message).toBe('Not found')
      expect((err as ApiError).status).toBe(404)
    }
  })

  it('throws ApiError with fallback message when body is not JSON', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('Internal Server Error', { status: 500 }),
    )

    try {
      await request('/admin/accounts')
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError)
      expect((err as ApiError).message).toBe('Unknown error')
      expect((err as ApiError).status).toBe(500)
    }
  })
})
