import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatusBadge } from '@/components/StatusBadge'

describe('StatusBadge', () => {
  it('renders active badge with green styling', () => {
    render(<StatusBadge status="active" />)
    const badge = screen.getByText('active')
    expect(badge).toBeInTheDocument()
    expect(badge.className).toContain('bg-green')
  })

  it('renders cooldown badge with yellow styling', () => {
    render(<StatusBadge status="cooldown" />)
    const badge = screen.getByText('cooldown')
    expect(badge).toBeInTheDocument()
    expect(badge.className).toContain('bg-yellow')
  })

  it('renders disabled badge with red styling', () => {
    render(<StatusBadge status="disabled" />)
    const badge = screen.getByText('disabled')
    expect(badge).toBeInTheDocument()
    expect(badge.className).toContain('bg-red')
  })
})
