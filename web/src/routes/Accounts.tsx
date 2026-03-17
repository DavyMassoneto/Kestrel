import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { AccountCard } from '@/components/AccountCard'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import {
  useAccounts,
  useDeleteAccount,
  useResetAccount,
} from '@/hooks/useAccounts'
import type { Account } from '@/types/account'

export default function Accounts() {
  const navigate = useNavigate()
  const { data: accounts, isLoading, error } = useAccounts()
  const deleteAccount = useDeleteAccount()
  const resetAccount = useResetAccount()

  const [confirmDelete, setConfirmDelete] = useState<Account | null>(null)
  const [confirmReset, setConfirmReset] = useState<Account | null>(null)

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
        Failed to load accounts: {error.message}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Accounts</h1>
        <Button onClick={() => navigate('/accounts/new')}>
          <Plus className="mr-2 h-4 w-4" />
          Add Account
        </Button>
      </div>

      {accounts?.length === 0 ? (
        <p className="py-10 text-center text-muted-foreground">
          No accounts configured yet.
        </p>
      ) : (
        <div className="space-y-3">
          {accounts?.map((account) => (
            <AccountCard
              key={account.id}
              account={account}
              onEdit={(a) =>
                navigate('/accounts/edit', { state: { account: a } })
              }
              onReset={(a) => setConfirmReset(a)}
              onDelete={(a) => setConfirmDelete(a)}
            />
          ))}
        </div>
      )}

      <ConfirmDialog
        open={!!confirmDelete}
        onOpenChange={(open) => !open && setConfirmDelete(null)}
        title="Delete Account"
        description={`Are you sure you want to delete "${confirmDelete?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        loading={deleteAccount.isPending}
        onConfirm={async () => {
          if (confirmDelete) {
            await deleteAccount.mutateAsync(confirmDelete.id)
            setConfirmDelete(null)
          }
        }}
      />

      <ConfirmDialog
        open={!!confirmReset}
        onOpenChange={(open) => !open && setConfirmReset(null)}
        title="Reset Account"
        description={`Reset cooldown and errors for "${confirmReset?.name}"?`}
        confirmLabel="Reset"
        loading={resetAccount.isPending}
        onConfirm={async () => {
          if (confirmReset) {
            await resetAccount.mutateAsync(confirmReset.id)
            setConfirmReset(null)
          }
        }}
      />
    </div>
  )
}
