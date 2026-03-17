import { useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { LogTable } from '@/components/LogTable'
import { useLogs } from '@/hooks/useLogs'
import type { LogFilters } from '@/types/requestLog'

const PAGE_SIZE = 50

export default function Logs() {
  const [filters, setFilters] = useState<LogFilters>({
    limit: PAGE_SIZE,
    offset: 0,
  })

  const [modelFilter, setModelFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [fromFilter, setFromFilter] = useState('')
  const [toFilter, setToFilter] = useState('')

  const { data, isLoading, error } = useLogs(filters)

  function applyFilters() {
    setFilters({
      limit: PAGE_SIZE,
      offset: 0,
      ...(modelFilter ? { model: modelFilter } : {}),
      ...(statusFilter ? { status: Number(statusFilter) } : {}),
      ...(fromFilter ? { from: fromFilter } : {}),
      ...(toFilter ? { to: toFilter } : {}),
    })
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') applyFilters()
  }

  const total = data?.total ?? 0
  const offset = filters.offset ?? 0
  const hasNext = offset + PAGE_SIZE < total
  const hasPrev = offset > 0

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Request Logs</h1>

      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            Model
          </label>
          <Input
            value={modelFilter}
            onChange={(e) => setModelFilter(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="claude-sonnet-4-5"
            className="w-48"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            Status
          </label>
          <Input
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="200"
            className="w-24"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            From
          </label>
          <Input
            type="datetime-local"
            value={fromFilter}
            onChange={(e) => setFromFilter(e.target.value)}
            className="w-48"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            To
          </label>
          <Input
            type="datetime-local"
            value={toFilter}
            onChange={(e) => setToFilter(e.target.value)}
            className="w-48"
          />
        </div>
        <Button onClick={applyFilters}>Search</Button>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
        </div>
      ) : error ? (
        <div className="py-10 text-center text-destructive">
          Failed to load logs: {error.message}
        </div>
      ) : (
        <>
          <LogTable logs={data?.data ?? []} />

          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              Showing {offset + 1}-{Math.min(offset + PAGE_SIZE, total)} of{' '}
              {total}
            </p>
            <div className="flex gap-1">
              <Button
                variant="outline"
                size="icon"
                disabled={!hasPrev}
                onClick={() =>
                  setFilters((f) => ({
                    ...f,
                    offset: Math.max(0, (f.offset ?? 0) - PAGE_SIZE),
                  }))
                }
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                disabled={!hasNext}
                onClick={() =>
                  setFilters((f) => ({
                    ...f,
                    offset: (f.offset ?? 0) + PAGE_SIZE,
                  }))
                }
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
