export type AccountStatus = 'active' | 'cooldown' | 'disabled'

export interface Account {
  id: string
  name: string
  base_url: string
  status: AccountStatus
  priority: number
  cooldown_until: string | null
  backoff_level: number
  last_used_at: string | null
  last_error: string | null
  created_at: string
}

export interface CreateAccountInput {
  name: string
  api_key: string
  base_url: string
  priority: number
}

export interface UpdateAccountInput {
  name?: string
  api_key?: string
  base_url?: string
  priority?: number
}
