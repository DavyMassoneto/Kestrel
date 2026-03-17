import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useCreateAccount, useUpdateAccount } from '@/hooks/useAccounts'
import type { Account } from '@/types/account'

export default function AccountForm() {
  const navigate = useNavigate()
  const location = useLocation()
  const editing = location.state?.account as Account | undefined

  const [name, setName] = useState(editing?.name ?? '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState(
    editing?.base_url ?? 'https://api.anthropic.com',
  )
  const [priority, setPriority] = useState(String(editing?.priority ?? 0))
  const [error, setError] = useState<string | null>(null)

  const create = useCreateAccount()
  const update = useUpdateAccount()

  const isEdit = !!editing

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    if (!name.trim()) {
      setError('Name is required')
      return
    }
    if (!isEdit && !apiKey.trim()) {
      setError('API key is required for new accounts')
      return
    }

    try {
      if (isEdit) {
        await update.mutateAsync({
          id: editing.id,
          input: {
            name: name.trim(),
            ...(apiKey.trim() ? { api_key: apiKey.trim() } : {}),
            base_url: baseUrl.trim(),
            priority: Number(priority),
          },
        })
      } else {
        await create.mutateAsync({
          name: name.trim(),
          api_key: apiKey.trim(),
          base_url: baseUrl.trim(),
          priority: Number(priority),
        })
      }
      navigate('/accounts')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save account')
    }
  }

  const isPending = create.isPending || update.isPending

  return (
    <div className="mx-auto max-w-md space-y-6">
      <h1 className="text-2xl font-bold">
        {isEdit ? 'Edit Account' : 'Add Account'}
      </h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="name">Name</Label>
          <Input
            id="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="claude-pro-1"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="apiKey">
            API Key {isEdit && '(leave blank to keep current)'}
          </Label>
          <Input
            id="apiKey"
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="sk-ant-api03-..."
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="baseUrl">Base URL</Label>
          <Input
            id="baseUrl"
            value={baseUrl}
            onChange={(e) => setBaseUrl(e.target.value)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="priority">Priority</Label>
          <Input
            id="priority"
            type="number"
            min="0"
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
        </div>

        {error && (
          <p className="text-sm text-destructive">{error}</p>
        )}

        <div className="flex gap-2">
          <Button type="submit" disabled={isPending}>
            {isPending ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => navigate('/accounts')}
          >
            Cancel
          </Button>
        </div>
      </form>
    </div>
  )
}
