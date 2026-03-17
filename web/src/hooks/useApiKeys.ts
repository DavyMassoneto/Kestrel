import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createKey, deleteKey, getKeys } from '@/api/keys'
import type { CreateAPIKeyInput } from '@/types/apiKey'

export function useApiKeys() {
  return useQuery({
    queryKey: ['apiKeys'],
    queryFn: () => getKeys().then((r) => r.data),
  })
}

export function useCreateApiKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateAPIKeyInput) => createKey(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['apiKeys'] }),
  })
}

export function useRevokeApiKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['apiKeys'] }),
  })
}
