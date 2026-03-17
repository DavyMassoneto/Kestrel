function classify(status: number) {
  if (status >= 200 && status < 300)
    return { label: 'success', cls: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' }
  if (status >= 400 && status < 500)
    return { label: 'warning', cls: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400' }
  return { label: 'error', cls: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' }
}

export function HttpStatusBadge({ status }: { status: number }) {
  const { cls } = classify(status)
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      {status}
    </span>
  )
}
