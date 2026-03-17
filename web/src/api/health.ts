import type { HealthResponse } from '@/types/health'
import { request } from './client'

export function getHealth(): Promise<HealthResponse> {
  return request('/health')
}
