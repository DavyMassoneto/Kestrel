import type {
  LogFilters,
  PaginatedResponse,
  RequestLog,
} from '@/types/requestLog'
import { request } from './client'

export function getLogs(
  filters?: LogFilters,
): Promise<PaginatedResponse<RequestLog>> {
  const params = new URLSearchParams()
  if (filters) {
    for (const [key, value] of Object.entries(filters)) {
      if (value !== undefined && value !== null) {
        params.set(key, String(value))
      }
    }
  }
  const qs = params.toString()
  return request(`/admin/logs${qs ? `?${qs}` : ''}`)
}
