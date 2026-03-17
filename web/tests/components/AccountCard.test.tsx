import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AccountCard } from '@/components/AccountCard'
import type { Account } from '@/types/account'

const baseAccount: Account = {
  id: 'acc_001',
  name: 'claude-pro-1',
  base_url: 'https://api.anthropic.com',
  status: 'active',
  priority: 0,
  cooldown_until: null,
  backoff_level: 0,
  last_used_at: new Date().toISOString(),
  last_error: null,
  created_at: '2026-01-01T00:00:00Z',
}

describe('AccountCard', () => {
  it('displays account name and status', () => {
    render(
      <AccountCard
        account={baseAccount}
        onEdit={vi.fn()}
        onReset={vi.fn()}
        onDelete={vi.fn()}
      />,
    )
    expect(screen.getByText('claude-pro-1')).toBeInTheDocument()
    expect(screen.getByText('active')).toBeInTheDocument()
    expect(screen.getByText('P:0')).toBeInTheDocument()
  })

  it('shows last error when present', () => {
    const account = { ...baseAccount, status: 'disabled' as const, last_error: '429 rate limit' }
    render(
      <AccountCard
        account={account}
        onEdit={vi.fn()}
        onReset={vi.fn()}
        onDelete={vi.fn()}
      />,
    )
    expect(screen.getByText('Error: 429 rate limit')).toBeInTheDocument()
  })

  it('calls onEdit when edit button is clicked', async () => {
    const onEdit = vi.fn()
    render(
      <AccountCard
        account={baseAccount}
        onEdit={onEdit}
        onReset={vi.fn()}
        onDelete={vi.fn()}
      />,
    )
    await userEvent.click(screen.getByTitle('Edit'))
    expect(onEdit).toHaveBeenCalledWith(baseAccount)
  })

  it('calls onReset when reset button is clicked', async () => {
    const onReset = vi.fn()
    render(
      <AccountCard
        account={baseAccount}
        onEdit={vi.fn()}
        onReset={onReset}
        onDelete={vi.fn()}
      />,
    )
    await userEvent.click(screen.getByTitle('Reset'))
    expect(onReset).toHaveBeenCalledWith(baseAccount)
  })

  it('calls onDelete when delete button is clicked', async () => {
    const onDelete = vi.fn()
    render(
      <AccountCard
        account={baseAccount}
        onEdit={vi.fn()}
        onReset={vi.fn()}
        onDelete={onDelete}
      />,
    )
    await userEvent.click(screen.getByTitle('Delete'))
    expect(onDelete).toHaveBeenCalledWith(baseAccount)
  })
})
