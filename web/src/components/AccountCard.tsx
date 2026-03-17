import { Pencil, RotateCcw, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/StatusBadge'
import type { Account } from '@/types/account'

function formatRelativeTime(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  const diff = Date.now() - new Date(dateStr).getTime()
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

interface AccountCardProps {
  account: Account
  onEdit: (account: Account) => void
  onReset: (account: Account) => void
  onDelete: (account: Account) => void
}

export function AccountCard({
  account,
  onEdit,
  onReset,
  onDelete,
}: AccountCardProps) {
  return (
    <div className="rounded-lg border bg-card p-4 text-card-foreground">
      <div className="flex items-start justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="font-semibold">{account.name}</span>
            <StatusBadge status={account.status} />
            <span className="text-xs text-muted-foreground">
              P:{account.priority}
            </span>
          </div>
          <p className="text-xs text-muted-foreground">
            Last used: {formatRelativeTime(account.last_used_at)}
          </p>
          {account.status === 'cooldown' && account.cooldown_until && (
            <p className="text-xs text-yellow-600 dark:text-yellow-400">
              Cooldown until:{' '}
              {new Date(account.cooldown_until).toLocaleTimeString()}
            </p>
          )}
          {account.last_error && (
            <p className="text-xs text-red-600 dark:text-red-400">
              Error: {account.last_error}
            </p>
          )}
        </div>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onEdit(account)}
            title="Edit"
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onReset(account)}
            title="Reset"
          >
            <RotateCcw className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onDelete(account)}
            title="Delete"
          >
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      </div>
    </div>
  )
}
