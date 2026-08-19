import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TranscriptTruncationDialog } from './transcript-truncation-dialog'

describe('TranscriptTruncationDialog', () => {
  it('confirms processing the complete recent messages', async () => {
    // Given a truncation warning
    const onConfirm = vi.fn()
    const onDecline = vi.fn()
    const user = userEvent.setup()
    render(<TranscriptTruncationDialog open onConfirm={onConfirm} onDecline={onDecline} />)

    // When accepting it
    await user.click(screen.getByRole('button', { name: 'Sim' }))

    // Then extraction is confirmed without declining
    expect(onConfirm).toHaveBeenCalledOnce()
    expect(onDecline).not.toHaveBeenCalled()
  })

  it('declines processing through the explicit negative action', async () => {
    // Given a truncation warning
    const onConfirm = vi.fn()
    const onDecline = vi.fn()
    const user = userEvent.setup()
    render(<TranscriptTruncationDialog open onConfirm={onConfirm} onDecline={onDecline} />)

    // When declining it
    await user.click(screen.getByRole('button', { name: 'Não' }))

    // Then extraction is declined without confirmation
    expect(onDecline).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('declines processing when dismissed through the dialog close control', async () => {
    // Given a truncation warning
    const onDecline = vi.fn()
    const user = userEvent.setup()
    render(<TranscriptTruncationDialog open onConfirm={vi.fn()} onDecline={onDecline} />)

    // When dismissing the dialog
    await user.click(screen.getByRole('button', { name: 'Close' }))

    // Then it follows the safe decline path
    expect(onDecline).toHaveBeenCalledOnce()
  })
})
