import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createAccount,
  deleteAccount,
  getAccounts,
  resetAccount,
  updateAccount,
} from '@/api/accounts'
import type { CreateAccountInput, UpdateAccountInput } from '@/types/account'

export function useAccounts() {
  return useQuery({
    queryKey: ['accounts'],
    queryFn: () => getAccounts().then((r) => r.data),
    refetchInterval: 5000,
  })
}

export function useCreateAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateAccountInput) => createAccount(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['accounts'] }),
  })
}

export function useUpdateAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateAccountInput }) =>
      updateAccount(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['accounts'] }),
  })
}

export function useDeleteAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteAccount(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['accounts'] }),
  })
}

export function useResetAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => resetAccount(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['accounts'] }),
  })
}
