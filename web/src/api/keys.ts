import type {
  APIKey,
  CreateAPIKeyInput,
  CreateAPIKeyResponse,
} from '@/types/apiKey'
import { request } from './client'

export function getKeys(): Promise<{ data: APIKey[] }> {
  return request('/admin/keys')
}

export function createKey(
  input: CreateAPIKeyInput,
): Promise<CreateAPIKeyResponse> {
  return request('/admin/keys', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function deleteKey(id: string): Promise<void> {
  return request(`/admin/keys/${id}`, { method: 'DELETE' })
}
