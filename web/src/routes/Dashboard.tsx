import {
  Activity,
  Clock,
  Hash,
  Users,
  Zap,
  AlertTriangle,
} from 'lucide-react'
import { StatsCard } from '@/components/StatsCard'
import { HttpStatusBadge } from '@/components/HttpStatusBadge'
import { useHealth } from '@/hooks/useHealth'
import { useAccounts } from '@/hooks/useAccounts'
import { useLogs } from '@/hooks/useLogs'
import type { AccountStatus } from '@/types/account'

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function formatNumber(n: number): string {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

export default function Dashboard() {
  const health = useHealth()
  const accounts = useAccounts()
  const logs = useLogs({ limit: 10 })

  const accountsByStatus = (accounts.data ?? []).reduce(
    (acc, a) => {
      acc[a.status] = (acc[a.status] ?? 0) + 1
      return acc
    },
    {} as Record<AccountStatus, number>,
  )

  const recentLogs = logs.data?.data ?? []
  const totalRequests = logs.data?.total ?? 0
  const totalTokens = recentLogs.reduce(
    (sum, l) => sum + l.input_tokens + l.output_tokens,
    0,
  )

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatsCard
          title="Active"
          value={accountsByStatus.active ?? 0}
          icon={Users}
          className="border-green-200 dark:border-green-900/50"
        />
        <StatsCard
          title="Cooldown"
          value={accountsByStatus.cooldown ?? 0}
          icon={AlertTriangle}
          className="border-yellow-200 dark:border-yellow-900/50"
        />
        <StatsCard
          title="Disabled"
          value={accountsByStatus.disabled ?? 0}
          icon={Zap}
          className="border-red-200 dark:border-red-900/50"
        />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatsCard
          title="Requests"
          value={formatNumber(totalRequests)}
          icon={Hash}
        />
        <StatsCard
          title="Tokens (recent)"
          value={formatNumber(totalTokens)}
          icon={Activity}
        />
        <StatsCard
          title="Uptime"
          value={health.data ? formatUptime(health.data.uptime_seconds) : '--'}
          icon={Clock}
        />
      </div>

      <div>
        <h2 className="mb-3 text-lg font-semibold">Recent Requests</h2>
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">ID</th>
                <th className="px-4 py-2 text-left font-medium">Model</th>
                <th className="px-4 py-2 text-left font-medium">Account</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-right font-medium">Latency</th>
              </tr>
            </thead>
            <tbody>
              {recentLogs.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-4 py-8 text-center text-muted-foreground"
                  >
                    No requests yet
                  </td>
                </tr>
              ) : (
                recentLogs.map((log) => (
                  <tr key={log.id} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-xs">
                      {log.id.slice(0, 12)}
                    </td>
                    <td className="px-4 py-2">{log.model}</td>
                    <td className="px-4 py-2">{log.account_name}</td>
                    <td className="px-4 py-2">
                      <HttpStatusBadge status={log.status} />
                    </td>
                    <td className="px-4 py-2 text-right font-mono text-xs">
                      {(log.latency_ms / 1000).toFixed(1)}s
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
