import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ResetLocalDataDialog } from './reset-local-data-dialog'

describe('ResetLocalDataDialog', () => {
  it('is closed when open is false', () => {
    // Given the dialog is closed
    render(
      <ResetLocalDataDialog open={false} pending={false} onCancel={vi.fn()} onConfirm={vi.fn()} />,
    )

    // Then no dialog content is shown
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows what is deleted and what is preserved', () => {
    // Given the dialog is open
    render(<ResetLocalDataDialog open pending={false} onCancel={vi.fn()} onConfirm={vi.fn()} />)

    // Then the copy names both
    expect(screen.getByText('Reset local data?')).toBeInTheDocument()
    expect(screen.getByText(/every study session/)).toBeInTheDocument()
    expect(screen.getByText(/OpenRouter key and profile are not affected/)).toBeInTheDocument()
  })

  it('calls onConfirm when the destructive action is clicked', async () => {
    // Given an open dialog
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(<ResetLocalDataDialog open pending={false} onCancel={vi.fn()} onConfirm={onConfirm} />)

    // When confirming the reset
    await user.click(screen.getByRole('button', { name: 'Yes, reset local data' }))

    // Then the owner is notified
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('calls onCancel when Cancel is clicked', async () => {
    // Given an open dialog
    const onCancel = vi.fn()
    const user = userEvent.setup()
    render(<ResetLocalDataDialog open pending={false} onCancel={onCancel} onConfirm={vi.fn()} />)

    // When cancelling
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then the owner is notified without confirming
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('disables both actions and shows progress while pending', () => {
    // Given a reset in flight
    render(<ResetLocalDataDialog open pending onCancel={vi.fn()} onConfirm={vi.fn()} />)

    // Then the confirm button reports progress and both actions are disabled
    expect(screen.getByRole('button', { name: 'Resetting...' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
  })
})
