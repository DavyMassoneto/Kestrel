export interface RequestLog {
  id: string
  api_key_name: string
  account_name: string
  model: string
  status: number
  input_tokens: number
  output_tokens: number
  latency_ms: number
  retries: number
  stream: boolean
  created_at: string
}

export interface LogFilters {
  limit?: number
  offset?: number
  status?: number
  account_id?: string
  api_key_id?: string
  model?: string
  from?: string
  to?: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}
