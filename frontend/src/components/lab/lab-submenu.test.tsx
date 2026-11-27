import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LabSubmenu } from './lab-submenu'

describe('LabSubmenu', () => {
  it('lists Interview and Practice and marks the current one', () => {
    render(<LabSubmenu kind="PRACTICE" onSelect={vi.fn()} />)

    expect(screen.getByRole('button', { name: 'Practice' })).toHaveAttribute('aria-current', 'true')
    expect(screen.getByRole('button', { name: 'Interview' })).not.toHaveAttribute('aria-current')
  })

  it('reports the kind that was clicked', async () => {
    const onSelect = vi.fn()
    render(<LabSubmenu kind="INTERVIEW" onSelect={onSelect} />)

    await userEvent.click(screen.getByRole('button', { name: 'Practice' }))

    expect(onSelect).toHaveBeenCalledExactlyOnceWith('PRACTICE')
  })
})
