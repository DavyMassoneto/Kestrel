import { HttpStatusBadge } from '@/components/HttpStatusBadge'
import type { RequestLog } from '@/types/requestLog'

function formatTime(dateStr: string | null | undefined): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

interface LogTableProps {
  logs: RequestLog[]
}

export function LogTable({ logs }: LogTableProps) {
  if (logs.length === 0) {
    return (
      <div className="py-10 text-center text-muted-foreground">
        No logs found
      </div>
    )
  }

  return (
    <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="px-4 py-2 text-left font-medium">ID</th>
            <th className="px-4 py-2 text-left font-medium">Model</th>
            <th className="px-4 py-2 text-left font-medium">Status</th>
            <th className="px-4 py-2 text-right font-medium">Tokens</th>
            <th className="px-4 py-2 text-right font-medium">Latency</th>
            <th className="px-4 py-2 text-left font-medium">Account</th>
            <th className="px-4 py-2 text-left font-medium">Key</th>
            <th className="px-4 py-2 text-center font-medium">Stream</th>
            <th className="px-4 py-2 text-left font-medium">Time</th>
          </tr>
        </thead>
        <tbody>
          {logs.map((log) => (
            <tr key={log.id} className="border-b last:border-0">
              <td className="px-4 py-2 font-mono text-xs">
                {log.id.slice(0, 12)}
              </td>
              <td className="px-4 py-2">{log.model}</td>
              <td className="px-4 py-2">
                <HttpStatusBadge status={log.status} />
              </td>
              <td className="px-4 py-2 text-right font-mono text-xs">
                {log.input_tokens + log.output_tokens > 0
                  ? `${log.input_tokens}/${log.output_tokens}`
                  : '--'}
              </td>
              <td className="px-4 py-2 text-right font-mono text-xs">
                {(log.latency_ms / 1000).toFixed(1)}s
              </td>
              <td className="px-4 py-2 text-xs">{log.account_name}</td>
              <td className="px-4 py-2 text-xs">{log.api_key_name}</td>
              <td className="px-4 py-2 text-center text-xs">
                {log.stream ? 'Yes' : 'No'}
              </td>
              <td className="px-4 py-2 text-xs text-muted-foreground">
                {formatTime(log.created_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
