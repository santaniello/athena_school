import { describe, expect, it, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { onFormattingProgress, type FormattingProgress } from '@/lib/ingest'
import { FormattingNoteDialog } from './formatting-note-dialog'

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return {
    ...original,
    onFormattingProgress: vi.fn(),
  }
})

function setupSubscription() {
  let progressHandler: (progress: FormattingProgress) => void = () => {}
  const unsubscribe = vi.fn()
  vi.mocked(onFormattingProgress).mockImplementation((handler) => {
    progressHandler = handler
    return unsubscribe
  })
  return {
    emitProgress: (progress: FormattingProgress) => act(() => progressHandler(progress)),
    unsubscribe,
  }
}

describe('FormattingNoteDialog', () => {
  it('shows a loading message while open', () => {
    // Given the dialog is open, with no progress tick yet
    setupSubscription()
    render(<FormattingNoteDialog open={true} />)

    // Then it explains what is happening with the plain, pre-progress message
    expect(screen.getByText('Formatting note…')).toBeInTheDocument()
    expect(
      screen.getByText('Reformatting your note and preparing the chunks it will create.'),
    ).toBeInTheDocument()
  })

  it('renders nothing when closed', () => {
    // Given the dialog is closed
    setupSubscription()
    render(<FormattingNoteDialog open={false} />)

    // Then its content is gone
    expect(screen.queryByText('Formatting note…')).not.toBeInTheDocument()
  })

  it('has no way to dismiss it — it closes only when the caller sets open to false', async () => {
    // Given the dialog is open
    setupSubscription()
    const user = userEvent.setup()
    render(<FormattingNoteDialog open={true} />)

    // When pressing Escape
    await user.keyboard('{Escape}')

    // Then it is still open — there is nothing to cancel, it closes on its
    // own once the preview resolves
    expect(screen.getByText('Formatting note…')).toBeInTheDocument()
  })

  it('shows "Formatting section X of Y" once a multi-batch progress tick arrives', () => {
    // Given the dialog is open and a tick reports the note split into
    // several formatting batches
    const { emitProgress } = setupSubscription()
    render(<FormattingNoteDialog open={true} />)

    // When the second of three batches finishes
    emitProgress({ sectionsProcessed: 2, sectionsTotal: 3 })

    // Then the description reflects per-section progress instead of the
    // generic message
    expect(screen.getByText('Formatting section 2 of 3…')).toBeInTheDocument()
  })

  it('keeps the generic message for a single-batch note, even once its one tick arrives', () => {
    // Given a note small enough to format in exactly one batch
    const { emitProgress } = setupSubscription()
    render(<FormattingNoteDialog open={true} />)

    // When that one batch finishes
    emitProgress({ sectionsProcessed: 1, sectionsTotal: 1 })

    // Then the description stays generic — "section 1 of 1" would tell the
    // user nothing a plain spinner doesn't
    expect(
      screen.getByText('Reformatting your note and preparing the chunks it will create.'),
    ).toBeInTheDocument()
  })

  it('unsubscribes and resets progress when closed', () => {
    // Given the dialog is open with a progress tick already shown
    const { emitProgress, unsubscribe } = setupSubscription()
    const { rerender } = render(<FormattingNoteDialog open={true} />)
    emitProgress({ sectionsProcessed: 1, sectionsTotal: 2 })
    expect(screen.getByText('Formatting section 1 of 2…')).toBeInTheDocument()

    // When the caller closes it
    rerender(<FormattingNoteDialog open={false} />)

    // Then it unsubscribed from further ticks
    expect(unsubscribe).toHaveBeenCalledOnce()
  })
})
