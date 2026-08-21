import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SourceModeSelect } from './source-mode-select'

describe('SourceModeSelect', () => {
  it('shows the current value', () => {
    // Given a select defaulted to notes
    render(<SourceModeSelect value="notes" onValueChange={vi.fn()} />)

    // Then the trigger shows "Notes"
    expect(screen.getByRole('combobox', { name: 'Source mode' })).toHaveTextContent('Notes')
  })

  it('offers Notes, Strict notes, and Web', async () => {
    // Given the select is open
    const user = userEvent.setup()
    render(<SourceModeSelect value="notes" onValueChange={vi.fn()} />)
    await user.click(screen.getByRole('combobox', { name: 'Source mode' }))
    const listbox = screen.getByRole('listbox')

    // Then all three modes are offered
    expect(within(listbox).getByText('Notes')).toBeInTheDocument()
    expect(within(listbox).getByText('Strict notes')).toBeInTheDocument()
    expect(within(listbox).getByText('Web')).toBeInTheDocument()
  })

  it('calls onValueChange with the picked mode', async () => {
    // Given the select is open
    const user = userEvent.setup()
    const onValueChange = vi.fn()
    render(<SourceModeSelect value="notes" onValueChange={onValueChange} />)
    await user.click(screen.getByRole('combobox', { name: 'Source mode' }))

    // When picking "Web"
    await user.click(within(screen.getByRole('listbox')).getByText('Web'))

    // Then the new mode is reported
    expect(onValueChange).toHaveBeenCalledWith('web')
  })

  it('is disabled when disabled is true', () => {
    // Given a disabled select
    render(<SourceModeSelect value="notes" onValueChange={vi.fn()} disabled />)

    // Then the trigger is disabled
    expect(screen.getByRole('combobox', { name: 'Source mode' })).toBeDisabled()
  })
})
