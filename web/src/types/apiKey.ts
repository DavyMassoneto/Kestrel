export interface APIKey {
  id: string
  name: string
  prefix: string
  is_active: boolean
  allowed_models: string[]
  created_at: string
  last_used_at: string | null
}

export interface CreateAPIKeyInput {
  name: string
  allowed_models: string[]
}

export interface CreateAPIKeyResponse {
  id: string
  key: string
  name: string
  allowed_models: string[]
}
