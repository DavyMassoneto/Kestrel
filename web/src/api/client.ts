const STORAGE_KEY = 'kestrel_auth'
const CURRENT_VERSION = 1

interface StoredAuth {
  version: number
  adminKey: string
}

export class AuthError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'AuthError'
  }
}

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export function getAdminKey(): string | null {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    const stored: StoredAuth = JSON.parse(raw)
    if (stored.version !== CURRENT_VERSION) {
      localStorage.removeItem(STORAGE_KEY)
      return null
    }
    return stored.adminKey
  } catch {
    localStorage.removeItem(STORAGE_KEY)
    return null
  }
}

export function setAdminKey(key: string): void {
  const stored: StoredAuth = { version: CURRENT_VERSION, adminKey: key }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(stored))
}

export function clearAdminKey(): void {
  localStorage.removeItem(STORAGE_KEY)
}

export async function request<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const adminKey = getAdminKey()

  const res = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(adminKey ? { 'X-Admin-Key': adminKey } : {}),
      ...options?.headers,
    },
  })

  if (res.status === 401 || res.status === 403) {
    window.dispatchEvent(new CustomEvent('kestrel:auth_error'))
    throw new AuthError('Admin key invalida ou ausente')
  }

  if (!res.ok) {
    let message = 'Unknown error'
    try {
      const body = await res.json()
      message = body.error?.message ?? message
    } catch {
      // response body not JSON
    }
    throw new ApiError(message, res.status)
  }

  return res.json() as Promise<T>
}
