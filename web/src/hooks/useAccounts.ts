import { useQuery } from '@tanstack/react-query'
import { getAccounts } from '@/api/accounts'

export function useAccounts() {
  return useQuery({
    queryKey: ['accounts'],
    queryFn: () => getAccounts().then((r) => r.data),
    refetchInterval: 5000,
  })
}
