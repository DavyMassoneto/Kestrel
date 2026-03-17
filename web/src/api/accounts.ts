import type {
  Account,
  CreateAccountInput,
  UpdateAccountInput,
} from '@/types/account'
import { request } from './client'

export function getAccounts(): Promise<{ data: Account[] }> {
  return request('/admin/accounts')
}

export function createAccount(
  input: CreateAccountInput,
): Promise<Account> {
  return request('/admin/accounts', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateAccount(
  id: string,
  input: UpdateAccountInput,
): Promise<Account> {
  return request(`/admin/accounts/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteAccount(id: string): Promise<void> {
  return request(`/admin/accounts/${id}`, { method: 'DELETE' })
}

export function resetAccount(id: string): Promise<Account> {
  return request(`/admin/accounts/${id}/reset`, { method: 'POST' })
}
