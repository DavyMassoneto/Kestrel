import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { useApiKeys, useRevokeApiKey } from '@/hooks/useApiKeys'
import type { APIKey } from '@/types/apiKey'

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

export default function ApiKeys() {
  const navigate = useNavigate()
  const { data: keys, isLoading, error } = useApiKeys()
  const revokeKey = useRevokeApiKey()
  const [confirmRevoke, setConfirmRevoke] = useState<APIKey | null>(null)

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="py-10 text-center text-destructive">
        Failed to load API keys: {error.message}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">API Keys</h1>
        <Button onClick={() => navigate('/keys/new')}>
          <Plus className="mr-2 h-4 w-4" />
          Create Key
        </Button>
      </div>

      {keys?.length === 0 ? (
        <p className="py-10 text-center text-muted-foreground">
          No API keys created yet.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">Name</th>
                <th className="px-4 py-2 text-left font-medium">Prefix</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-left font-medium">Models</th>
                <th className="px-4 py-2 text-left font-medium">Last Used</th>
                <th className="px-4 py-2 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {keys?.map((key) => (
                <tr key={key.id} className="border-b last:border-0">
                  <td className="px-4 py-2 font-medium">{key.name}</td>
                  <td className="px-4 py-2 font-mono text-xs">
                    {key.prefix}...
                  </td>
                  <td className="px-4 py-2">
                    <StatusBadge
                      status={key.is_active ? 'active' : 'disabled'}
                    />
                  </td>
                  <td className="px-4 py-2 text-xs">
                    {key.allowed_models.length === 0
                      ? 'All'
                      : key.allowed_models.join(', ')}
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">
                    {formatRelativeTime(key.last_used_at)}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive"
                      onClick={() => setConfirmRevoke(key)}
                    >
                      Revoke
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!confirmRevoke}
        onOpenChange={(open) => !open && setConfirmRevoke(null)}
        title="Revoke API Key"
        description={`Are you sure you want to revoke "${confirmRevoke?.name}"? This action cannot be undone.`}
        confirmLabel="Revoke"
        variant="destructive"
        loading={revokeKey.isPending}
        onConfirm={async () => {
          if (confirmRevoke) {
            await revokeKey.mutateAsync(confirmRevoke.id)
            setConfirmRevoke(null)
          }
        }}
      />
    </div>
  )
}
