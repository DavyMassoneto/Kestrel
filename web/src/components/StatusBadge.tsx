import type { AccountStatus } from '@/types/account'

const variants: Record<AccountStatus, string> = {
  active: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
  cooldown: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
  disabled: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
}

export function StatusBadge({ status }: { status: AccountStatus }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${variants[status]}`}>
      {status}
    </span>
  )
}
