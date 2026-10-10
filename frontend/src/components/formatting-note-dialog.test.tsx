import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { FormattingNoteDialog } from './formatting-note-dialog'

describe('FormattingNoteDialog', () => {
  it('shows a loading message while open', () => {
    // Given the dialog is open
    render(<FormattingNoteDialog open={true} />)

    // Then it explains what is happening
    expect(screen.getByText('Formatting note…')).toBeInTheDocument()
  })

  it('renders nothing when closed', () => {
    // Given the dialog is closed
    render(<FormattingNoteDialog open={false} />)

    // Then its content is gone
    expect(screen.queryByText('Formatting note…')).not.toBeInTheDocument()
  })

  it('has no way to dismiss it — it closes only when the caller sets open to false', async () => {
    // Given the dialog is open
    const user = userEvent.setup()
    render(<FormattingNoteDialog open={true} />)

    // When pressing Escape
    await user.keyboard('{Escape}')

    // Then it is still open — there is nothing to cancel, it closes on its
    // own once the preview resolves
    expect(screen.getByText('Formatting note…')).toBeInTheDocument()
  })
})
