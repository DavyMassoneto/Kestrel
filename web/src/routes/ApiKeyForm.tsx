import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Copy, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useCreateApiKey } from '@/hooks/useApiKeys'

export default function ApiKeyForm() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [modelsInput, setModelsInput] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const create = useCreateApiKey()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    if (!name.trim()) {
      setError('Name is required')
      return
    }

    const allowed_models = modelsInput
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)

    try {
      const result = await create.mutateAsync({
        name: name.trim(),
        allowed_models,
      })
      setCreatedKey(result.key)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create key')
    }
  }

  async function handleCopy() {
    if (createdKey) {
      await navigator.clipboard.writeText(createdKey)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  if (createdKey) {
    return (
      <div className="mx-auto max-w-md space-y-6">
        <h1 className="text-2xl font-bold">API Key Created</h1>
        <div className="rounded-lg border border-yellow-300 bg-yellow-50 p-4 dark:border-yellow-800 dark:bg-yellow-900/20">
          <p className="mb-2 text-sm font-medium text-yellow-800 dark:text-yellow-200">
            Copy this key now. You won't be able to see it again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 break-all rounded bg-background px-2 py-1 text-sm">
              {createdKey}
            </code>
            <Button variant="outline" size="icon" onClick={handleCopy}>
              {copied ? (
                <Check className="h-4 w-4 text-green-600" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
        <Button onClick={() => navigate('/keys')}>Done</Button>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <h1 className="text-2xl font-bold">Create API Key</h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="name">Name</Label>
          <Input
            id="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="claude-code-main"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="models">
            Allowed Models (comma-separated, empty = all)
          </Label>
          <Input
            id="models"
            value={modelsInput}
            onChange={(e) => setModelsInput(e.target.value)}
            placeholder="claude-sonnet-4-5, claude-opus-4-5"
          />
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}

        <div className="flex gap-2">
          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? 'Creating...' : 'Create'}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => navigate('/keys')}
          >
            Cancel
          </Button>
        </div>
      </form>
    </div>
  )
}
