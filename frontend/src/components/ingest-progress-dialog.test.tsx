import { describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  importFile,
  importNotes,
  onIngestDone,
  onIngestError,
  onIngestProgress,
  type IngestProgress,
  type IngestSummary,
} from '@/lib/ingest'
import { IngestProgressDialog } from './ingest-progress-dialog'

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return {
    ...original,
    importNotes: vi.fn(),
    importFile: vi.fn(),
    onIngestProgress: vi.fn(),
    onIngestDone: vi.fn(),
    onIngestError: vi.fn(),
  }
})

function setupSubscriptions() {
  let progressHandler: (progress: IngestProgress) => void = () => {}
  let doneHandler: (summary: IngestSummary) => void = () => {}
  let errorHandler: (message: string) => void = () => {}
  const unsubscribeProgress = vi.fn()
  const unsubscribeDone = vi.fn()
  const unsubscribeError = vi.fn()

  vi.mocked(onIngestProgress).mockImplementation((handler) => {
    progressHandler = handler
    return unsubscribeProgress
  })
  vi.mocked(onIngestDone).mockImplementation((handler) => {
    doneHandler = handler
    return unsubscribeDone
  })
  vi.mocked(onIngestError).mockImplementation((handler) => {
    errorHandler = handler
    return unsubscribeError
  })

  return {
    emitProgress: (progress: IngestProgress) => act(() => progressHandler(progress)),
    emitDone: (summary: IngestSummary) => act(() => doneHandler(summary)),
    emitError: (message: string) => act(() => errorHandler(message)),
    unsubscribeProgress,
    unsubscribeDone,
    unsubscribeError,
  }
}

// Once finished, the footer's explicit "Close" button and the dialog's
// built-in X icon control (see ui/dialog.tsx's sr-only "Close" label) both
// carry the accessible name "Close" — findByRole/getByRole would be
// ambiguous, so tests that need the footer button specifically go through
// this helper instead. It always renders first (see DialogContent: children
// before the X button), so index 0 is stable.
function findFooterCloseButton() {
  return screen.findAllByRole('button', { name: 'Close' }).then((buttons) => buttons[0])
}

const emptySummary: IngestSummary = {
  filesScanned: 10,
  filesIngested: 8,
  filesSkipped: 2,
  filesFailed: 0,
  chunksCreated: 24,
  failures: [],
  indexWarnings: [],
}

describe('IngestProgressDialog', () => {
  it('starts the import for the given folder as soon as it opens', () => {
    // Given a dialog opened for a chosen folder
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))

    // When rendering it open
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // Then the import starts immediately for that folder, and the dialog
    // explains that it is still processing
    expect(importNotes).toHaveBeenCalledWith('/home/user/notes')
    expect(screen.getByText('Processing files in the selected folder.')).toBeInTheDocument()
    void events
  })

  it('does not start an import while closed', () => {
    // Given a dialog that is not open
    setupSubscriptions()

    // When rendering it closed
    render(
      <IngestProgressDialog open={false} kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )

    // Then no import is started
    expect(importNotes).not.toHaveBeenCalled()
  })

  it('shows live progress as ingest:progress events arrive', () => {
    // Given a dialog mid-import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // When a progress event arrives
    events.emitProgress({
      filesProcessed: 3,
      filesTotal: 10,
      chunksCreated: 9,
      currentFile: 'go/channels.md',
    })

    // Then the current progress and file are shown, no error alert, and
    // the progress bar reflects the exact 30% completion (3 of 10)
    expect(screen.getByText('3 of 10 files')).toBeInTheDocument()
    expect(screen.getByText('go/channels.md')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    const indicator = document.querySelector('[data-slot="progress-indicator"]')
    expect(indicator).toHaveStyle({ transform: 'translateX(-70%)' })
  })

  it('keeps the progress bar at zero without dividing by zero when the file total has not arrived yet', () => {
    // Given a dialog mid-import, before any files have been counted
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // When a progress event arrives reporting zero files total
    events.emitProgress({
      filesProcessed: 0,
      filesTotal: 0,
      chunksCreated: 0,
      currentFile: '',
    })

    // Then the bar stays at a clean 0% instead of computing 0/0
    const indicator = document.querySelector('[data-slot="progress-indicator"]')
    expect(indicator).toHaveStyle({ transform: 'translateX(-100%)' })
  })

  it('transitions to the result summary when ingest:done fires', async () => {
    // Given a dialog mid-import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // When the import finishes
    events.emitDone(emptySummary)

    // Then the summary counts replace the progress bar, a manual close
    // action becomes available, and with zero failures no failures list
    // renders at all
    expect(screen.getByText('Import complete.')).toBeInTheDocument()
    expect(screen.getByText('8')).toBeInTheDocument()
    expect(screen.queryByText('Starting...')).not.toBeInTheDocument()
    expect(await findFooterCloseButton()).toBeInTheDocument()
    expect(document.querySelector('.thin-scroll')).not.toBeInTheDocument()
  })

  it('lists per-file failures with their reason', () => {
    // Given a dialog mid-import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // When the import finishes with one failure
    events.emitDone({
      ...emptySummary,
      filesFailed: 1,
      failures: [{ path: 'notes/broken.md', reason: 'invalid utf-8' }],
    })

    // Then the failing path and its reason are both shown
    expect(screen.getByText('notes/broken.md')).toBeInTheDocument()
    expect(screen.getByText(/invalid utf-8/)).toBeInTheDocument()
  })

  it('lists files that imported successfully but need a knowledge index retry', () => {
    // Given a dialog mid-import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // When the import finishes with one file that persisted but whose
    // index reconciliation failed
    events.emitDone({
      ...emptySummary,
      indexWarnings: [{ path: 'notes/go.md', reason: 'store exploded' }],
    })

    // Then the file is listed, distinctly from a real failure
    expect(screen.getByText('notes/go.md')).toBeInTheDocument()
    expect(screen.getByText(/retry the knowledge index/)).toBeInTheDocument()
  })

  it('shows the error state when ingest:error fires instead of ingest:done', async () => {
    // Given a dialog mid-import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // When the whole import fails outright
    events.emitError('the folder does not exist')

    // Then the error is shown and closing becomes available
    expect(screen.getByText('the folder does not exist')).toBeInTheDocument()
    expect(await findFooterCloseButton()).toBeInTheDocument()
  })

  it('falls back to a closable error state when importNotes rejects without an ingest:error event', async () => {
    // Given an import whose binding call itself fails before ever
    // emitting ingest:error (e.g. an IPC-level failure)
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(Promise.reject(new Error('IPC failure')))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // Then a generic error is shown and the dialog still becomes closable
    expect(await screen.findByText('Failed to import notes. Please try again.')).toBeInTheDocument()
    expect(await findFooterCloseButton()).toBeInTheDocument()
    void events
  })

  it('ignores a stale rejection from a closed-and-reopened import for another folder', async () => {
    // Given an import for one folder that has not settled yet
    const events = setupSubscriptions()
    let rejectFirst: (error: Error) => void = () => {}
    const firstImport = new Promise<void>((_resolve, reject) => {
      rejectFirst = reject
    })
    firstImport.catch(() => {}) // avoid an unhandled-rejection warning from this local reference
    vi.mocked(importNotes).mockReturnValueOnce(firstImport)
    const { rerender } = render(
      <IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )

    // When the dialog is closed, then reopened for a different folder
    // whose import is still pending, and only then does the first
    // folder's import reject
    rerender(
      <IngestProgressDialog open={false} kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    rerender(<IngestProgressDialog open kind="folder" path="/home/user/other" onClose={vi.fn()} />)
    await waitFor(() => expect(importNotes).toHaveBeenCalledWith('/home/user/other'))
    await act(async () => {
      rejectFirst(new Error('stale IPC failure'))
      await Promise.resolve()
    })

    // Then the new import's state is unaffected by the stale rejection
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByText('Starting...')).toBeInTheDocument()
    void events
  })

  it('has no way to dismiss it while the import is still running', () => {
    // Given a dialog mid-import
    setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />)

    // Then neither the dialog's own close (X) control nor the Close
    // action exists yet
    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
  })

  it('calls onClose when Close is clicked after finishing', async () => {
    // Given a finished import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={onClose} />)
    events.emitDone(emptySummary)

    // When clicking Close
    await user.click(await findFooterCloseButton())

    // Then the owner is notified
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('calls onClose when dismissed via Escape after finishing', async () => {
    // Given a finished import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={onClose} />)
    events.emitDone(emptySummary)
    await findFooterCloseButton()

    // When dismissing it with Escape instead of the Close button
    await user.keyboard('{Escape}')

    // Then the owner is still notified
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('never calls onClose from Escape while the import is still running', async () => {
    // Given a dialog mid-import
    setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(<IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={onClose} />)

    // When pressing Escape before it has finished
    await user.keyboard('{Escape}')

    // Then it is ignored
    expect(onClose).not.toHaveBeenCalled()
  })

  it('unsubscribes every event listener when it closes', () => {
    // Given an open dialog
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    const { rerender } = render(
      <IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )

    // When it closes
    rerender(
      <IngestProgressDialog open={false} kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )

    // Then every subscription is torn down
    expect(events.unsubscribeProgress).toHaveBeenCalled()
    expect(events.unsubscribeDone).toHaveBeenCalled()
    expect(events.unsubscribeError).toHaveBeenCalled()
  })

  it('resets to a fresh progress state when reopened for another folder', async () => {
    // Given a dialog that made progress, then finished one import
    const events = setupSubscriptions()
    vi.mocked(importNotes).mockReturnValue(new Promise<void>(() => {}))
    const { rerender } = render(
      <IngestProgressDialog open kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )
    events.emitProgress({
      filesProcessed: 3,
      filesTotal: 10,
      chunksCreated: 9,
      currentFile: 'go/channels.md',
    })
    events.emitDone(emptySummary)
    expect(await findFooterCloseButton()).toBeInTheDocument()

    // When it is closed then reopened for a new folder
    rerender(
      <IngestProgressDialog open={false} kind="folder" path="/home/user/notes" onClose={vi.fn()} />,
    )
    rerender(<IngestProgressDialog open kind="folder" path="/home/user/other" onClose={vi.fn()} />)

    // Then it starts a fresh import instead of showing the stale summary
    await waitFor(() => expect(importNotes).toHaveBeenCalledWith('/home/user/other'))
    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
    expect(screen.getByText('Starting...')).toBeInTheDocument()
  })

  describe('kind="file"', () => {
    it('starts the import through importFile for the given path as soon as it opens, with the file description', () => {
      // Given a dialog opened for a chosen file
      const events = setupSubscriptions()
      vi.mocked(importFile).mockReturnValueOnce(new Promise<void>(() => {}))

      // When rendering it open
      render(
        <IngestProgressDialog open kind="file" path="/home/user/notes/go.md" onClose={vi.fn()} />,
      )

      // Then the import starts immediately through importFile (not
      // importNotes), and the dialog explains it is processing the file
      expect(importFile).toHaveBeenCalledWith('/home/user/notes/go.md')
      expect(importNotes).not.toHaveBeenCalled()
      expect(screen.getByText('Processing the selected file.')).toBeInTheDocument()
      void events
    })

    it('shows 1 of 1 files like any other progress update', () => {
      // Given a dialog mid single-file import
      const events = setupSubscriptions()
      vi.mocked(importFile).mockReturnValueOnce(new Promise<void>(() => {}))
      render(
        <IngestProgressDialog open kind="file" path="/home/user/notes/go.md" onClose={vi.fn()} />,
      )

      // When the progress event for the one file arrives
      events.emitProgress({
        filesProcessed: 1,
        filesTotal: 1,
        chunksCreated: 2,
        currentFile: 'go.md',
      })

      // Then it renders with the same shared progress copy as folder import
      expect(screen.getByText('1 of 1 files')).toBeInTheDocument()
    })

    it('transitions to the result summary when ingest:done fires, same as folder import', () => {
      // Given a dialog mid single-file import
      const events = setupSubscriptions()
      vi.mocked(importFile).mockReturnValueOnce(new Promise<void>(() => {}))
      render(
        <IngestProgressDialog open kind="file" path="/home/user/notes/go.md" onClose={vi.fn()} />,
      )

      // When the import finishes
      events.emitDone(emptySummary)

      // Then the summary counts replace the progress bar
      expect(screen.getByText('Import complete.')).toBeInTheDocument()
    })

    it('falls back to a closable error state when importFile rejects without an ingest:error event', async () => {
      // Given a single-file import whose binding call itself fails before
      // ever emitting ingest:error
      const events = setupSubscriptions()
      vi.mocked(importFile).mockReturnValueOnce(Promise.reject(new Error('IPC failure')))
      render(
        <IngestProgressDialog open kind="file" path="/home/user/notes/go.md" onClose={vi.fn()} />,
      )

      // Then a generic error is shown and the dialog still becomes closable
      expect(
        await screen.findByText('Failed to import notes. Please try again.'),
      ).toBeInTheDocument()
      expect(await findFooterCloseButton()).toBeInTheDocument()
      void events
    })
  })
})
