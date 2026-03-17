import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getLogs } from '@/api/logs'
import type { LogFilters } from '@/types/requestLog'

export function useLogs(filters?: LogFilters) {
  return useQuery({
    queryKey: ['logs', filters],
    queryFn: () => getLogs(filters),
    placeholderData: keepPreviousData,
  })
}
