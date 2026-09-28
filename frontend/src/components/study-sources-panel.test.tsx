import { describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StudySourcesPanel } from './study-sources-panel'
import { getSessionSourceDocument, listSessionSources, removeSessionSource } from '@/lib/sources'
import {
  importFile,
  onIngestDone,
  onIngestError,
  onIngestProgress,
  pickNotesFile,
  type IngestSummary,
} from '@/lib/ingest'

vi.mock('@/lib/sources', () => ({
  listSessionSources: vi.fn(),
  removeSessionSource: vi.fn(),
  getSessionSourceDocument: vi.fn(),
}))

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return {
    ...original,
    pickNotesFile: vi.fn(),
    importFile: vi.fn(),
    onIngestProgress: vi.fn(),
    onIngestDone: vi.fn(),
    onIngestError: vi.fn(),
  }
})

// IngestProgressDialog (rendered for real, not mocked) only reaches its
// "finished" state via onIngestDone/onIngestError — importFile's own
// resolution just guards an unhandled rejection. Mirrors
// ingest-progress-dialog.test.tsx's own setupSubscriptions helper.
function setupIngestDialogSubscriptions() {
  let doneHandler: (summary: IngestSummary) => void = () => {}
  vi.mocked(onIngestProgress).mockReturnValue(vi.fn())
  vi.mocked(onIngestDone).mockImplementation((handler) => {
    doneHandler = handler
    return vi.fn()
  })
  vi.mocked(onIngestError).mockReturnValue(vi.fn())
  vi.mocked(importFile).mockReturnValue(new Promise(() => {}))
  return {
    emitDone: (summary: IngestSummary) => act(() => doneHandler(summary)),
  }
}

// Once finished, the dialog's footer "Close" button and its built-in X icon
// control both carry the accessible name "Close" — see
// ingest-progress-dialog.test.tsx's own findFooterCloseButton, mirrored
// here since IngestProgressDialog is rendered for real, not mocked.
function findFooterCloseButton() {
  return screen.findAllByRole('button', { name: 'Close' }).then((buttons) => buttons[0])
}

const EMPTY_SUMMARY: IngestSummary = {
  filesScanned: 1,
  filesIngested: 1,
  filesSkipped: 0,
  filesFailed: 0,
  chunksCreated: 3,
  failures: [],
  indexWarnings: [],
}

const DISTRIBUTED_SYSTEMS = {
  itemId: 'item-1',
  title: 'Distributed Systems',
  path: 'notes/ds.md',
  chunkCount: 12,
  ingestedAt: '2024-01-01T00:00:00Z',
}

const CAP_THEOREM = {
  itemId: 'item-2',
  title: 'CAP theorem',
  path: 'cap.md',
  chunkCount: 1,
  ingestedAt: '2024-01-02T00:00:00Z',
}

describe('StudySourcesPanel', () => {
  it('shows a loading state before the sources arrive', () => {
    // Given a load that never resolves during this test
    vi.mocked(listSessionSources).mockImplementationOnce(() => new Promise(() => {}))

    // When the panel mounts
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then a loading message shows, not the empty state
    expect(screen.getByText('Loading sources…')).toBeInTheDocument()
    expect(screen.queryByText('No sources imported yet.')).not.toBeInTheDocument()
  })

  it('shows an empty state when the session has no imported documents', async () => {
    // Given a session with nothing imported
    vi.mocked(listSessionSources).mockResolvedValueOnce([])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then the empty state shows, with its own explanation and Add action
    expect(await screen.findByText('No sources imported yet.')).toBeInTheDocument()
    expect(
      screen.getByText('The chat searches these documents when answering.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add a source' })).toBeInTheDocument()
  })

  it('shows every imported document, oldest first, with its path and chunk count', async () => {
    // Given a session with two imported documents
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then both render with their title, path and chunk count, singular for one chunk
    expect(await screen.findByText('Distributed Systems')).toBeInTheDocument()
    expect(screen.getByText('notes/ds.md · 12 chunks')).toBeInTheDocument()
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.getByText('cap.md · 1 chunk')).toBeInTheDocument()
  })

  it('shows the header count for every loaded document', async () => {
    // Given two imported documents
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then the header shows the total count
    expect(await screen.findByText('Sources (2)')).toBeInTheDocument()
  })

  it('filters the list by title or path as the user types', async () => {
    // Given two imported documents, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching for the other document's title
    await user.type(screen.getByPlaceholderText('Search sources'), 'cap theorem')

    // Then only the matching document remains
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument()
  })

  it('matches the search against the path too', async () => {
    // Given two imported documents, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching for a path fragment
    await user.type(screen.getByPlaceholderText('Search sources'), 'notes/ds')

    // Then only the matching document remains
    expect(screen.getByText('Distributed Systems')).toBeInTheDocument()
    expect(screen.queryByText('CAP theorem')).not.toBeInTheDocument()
  })

  it('trims leading and trailing whitespace from the search query', async () => {
    // Given two imported documents, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching with surrounding whitespace around an otherwise
    // matching term
    await user.type(screen.getByPlaceholderText('Search sources'), '  CAP theorem  ')

    // Then it still matches, trimmed
    expect(screen.getByText('CAP theorem')).toBeInTheDocument()
    expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument()
  })

  it('shows a no-match message when the search matches nothing', async () => {
    // Given one imported document, loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When searching for something that matches nothing
    await user.type(screen.getByPlaceholderText('Search sources'), 'nonexistent')

    // Then a no-match message shows, distinct from the empty-session message
    expect(screen.getByText('No sources match your search.')).toBeInTheDocument()
  })

  it('shows an error with a retry action when loading fails', async () => {
    // Given a load that fails
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('database unavailable'))

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)

    // Then the error shows with a Retry action, instead of the list/empty state
    expect(await screen.findByText('database unavailable')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('retrying reloads the sources', async () => {
    // Given a load that failed once
    vi.mocked(listSessionSources).mockRejectedValueOnce(new Error('database unavailable'))
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByRole('button', { name: 'Retry' })

    // When retrying, and the second attempt succeeds
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    // Then the list shows
    expect(await screen.findByText('Distributed Systems')).toBeInTheDocument()
  })

  it('reloads when sessionId changes', async () => {
    // Given session-1's document loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const { rerender } = render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When the panel is handed a different session
    vi.mocked(listSessionSources).mockResolvedValueOnce([CAP_THEOREM])
    rerender(<StudySourcesPanel sessionId="session-2" />)

    // Then it loads and shows that session's documents instead
    expect(await screen.findByText('CAP theorem')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('Distributed Systems')).not.toBeInTheDocument())
    expect(listSessionSources).toHaveBeenCalledWith('session-2')
  })

  it('opens a confirmation naming the document when Remove is picked from its menu', async () => {
    // Given one document loaded
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When opening its ⋮ menu and picking Remove
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))

    // Then a confirmation names the document and explains the scope
    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Remove Distributed Systems?')).toBeInTheDocument()
    expect(
      within(dialog).getByText(
        'It will no longer be searched in this session. The file on your computer is not changed.',
      ),
    ).toBeInTheDocument()
    expect(dialog.querySelector('p.text-destructive')).toBeNull()
  })

  it('cancelling the confirmation removes nothing', async () => {
    // Given the confirmation open for one document
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('alertdialog')

    // When cancelling
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    // Then nothing was removed, and the confirmation closes
    expect(removeSessionSource).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })

  it('confirming Remove deletes the document and reloads the list', async () => {
    // Given the confirmation open for one document, and a removal that succeeds
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    vi.mocked(removeSessionSource).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('alertdialog')

    // When confirming
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))

    // Then it removed the right document, reloaded, and the confirmation closed
    expect(removeSessionSource).toHaveBeenCalledWith('session-1', 'item-1')
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(await screen.findByText('No sources imported yet.')).toBeInTheDocument()
    expect(listSessionSources).toHaveBeenCalledTimes(2)
  })

  it('a failed removal keeps the confirmation open and shows the error inside it', async () => {
    // Given the confirmation open for one document, and a removal that fails
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    vi.mocked(removeSessionSource).mockRejectedValueOnce(new Error('ingest: source not found'))
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('alertdialog')

    // When confirming and it fails
    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))

    // Then the confirmation stays open, showing the error, and the row survives
    expect(await within(dialog).findByText('ingest: source not found')).toBeInTheDocument()
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
    expect(screen.getByText('Distributed Systems')).toBeInTheDocument()
    expect(listSessionSources).toHaveBeenCalledOnce()
  })

  it('reopening the confirmation for another document clears a previous error', async () => {
    // Given a failed removal attempt on one document
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS, CAP_THEOREM])
    vi.mocked(removeSessionSource).mockRejectedValueOnce(new Error('ingest: source not found'))
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))
    await within(dialog).findByText('ingest: source not found')
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())

    // When opening the confirmation for a different document
    await user.click(screen.getByRole('button', { name: 'CAP theorem options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))

    // Then the stale error from the other document is gone
    expect(screen.queryByText('ingest: source not found')).not.toBeInTheDocument()
  })

  it('disables Cancel/Remove and ignores Escape while a removal is in flight', async () => {
    // Given a removal held pending until released
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    let resolveRemove: () => void = () => {}
    vi.mocked(removeSessionSource).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRemove = resolve
      }),
    )
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('alertdialog')

    // When confirming, while the removal is still pending
    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))

    // Then both actions are disabled, and Escape does not close it
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeDisabled()
    expect(within(dialog).getByRole('button', { name: 'Remove' })).toBeDisabled()
    await user.keyboard('{Escape}')
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()

    // Cleanup: release the pending removal (whose success reloads the list)
    // so it doesn't leak into another test
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    resolveRemove()
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })

  it('retrying Remove in the same confirmation clears the previous error on success', async () => {
    // Given a first Remove attempt that failed
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    vi.mocked(removeSessionSource).mockRejectedValueOnce(new Error('ingest: source not found'))
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByRole('button', { name: 'Distributed Systems options' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Remove' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))
    await within(dialog).findByText('ingest: source not found')

    // When retrying, in the same still-open confirmation, held pending
    let resolveRetry: () => void = () => {}
    vi.mocked(removeSessionSource).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRetry = resolve
      }),
    )
    await user.click(within(dialog).getByRole('button', { name: 'Remove' }))

    // Then no error shows at all, immediately, before the retry even
    // resolves — not just the old text swapped for something else, and not
    // just once the dialog eventually closes
    expect(dialog.querySelector('p.text-destructive')).toBeNull()

    // And once it succeeds, the confirmation closes
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    resolveRetry()
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
  })

  it("disables each row's ⋮ menu, with a reason, while the index is retrying", async () => {
    // Given the index is retrying
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" mutationsDisabled />)
    await screen.findByText('Distributed Systems')

    // Then the row's menu trigger is disabled, with the reason as a tooltip
    const optionsButton = screen.getByRole('button', { name: 'Distributed Systems options' })
    expect(optionsButton).toBeDisabled()
    await user.hover(optionsButton)
    expect(await screen.findByText('Rebuilding knowledge index…')).toBeInTheDocument()
  })

  it('Add opens the file picker, then the import dialog with the picked path', async () => {
    // Given a picker that resolves to a chosen file
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    vi.mocked(pickNotesFile).mockResolvedValueOnce('/home/user/notes/go.md')
    setupIngestDialogSubscriptions()
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')

    // When clicking Add
    await user.click(screen.getByRole('button', { name: 'Add source' }))

    // Then the import dialog opens for that session and path
    expect(await screen.findByText('Importing notes')).toBeInTheDocument()
    expect(importFile).toHaveBeenCalledWith('session-1', '/home/user/notes/go.md')
  })

  it('renders the header Add button with the default (primary) variant, unlike the outline empty-state one', async () => {
    // Given an empty session
    vi.mocked(listSessionSources).mockResolvedValueOnce([])

    // When the panel loads
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')

    // Then the header's Add button is styled as primary, distinct from the
    // empty state's outline "Add a source" button
    expect(screen.getByRole('button', { name: 'Add source' })).toHaveAttribute(
      'data-variant',
      'default',
    )
    expect(screen.getByRole('button', { name: 'Add a source' })).toHaveAttribute(
      'data-variant',
      'outline',
    )
  })

  it('never shows the retrying tooltip while Add is enabled', async () => {
    // Given Add is enabled (mutationsDisabled defaults to false)
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When hovering it
    await user.hover(screen.getByRole('button', { name: 'Add source' }))

    // Then no "retrying" tooltip ever appears
    expect(screen.queryByText('Rebuilding knowledge index…')).not.toBeInTheDocument()
  })

  it('shows no picker error on mount', () => {
    // Given a fresh panel (the load itself never resolves in this test)
    vi.mocked(listSessionSources).mockImplementationOnce(() => new Promise(() => {}))

    // When it mounts
    const { container } = render(<StudySourcesPanel sessionId="session-1" />)

    // Then no destructive-styled message renders at all — not even an empty one
    expect(container.querySelector('.text-destructive')).toBeNull()
  })

  it('clears a previous picker error once Add succeeds', async () => {
    // Given a first Add attempt whose picker failed
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    vi.mocked(pickNotesFile).mockRejectedValueOnce(new Error('dialog unavailable'))
    const user = userEvent.setup()
    const { container } = render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    await screen.findByText('Failed to open the notes picker. Please try again.')

    // When clicking Add again and the picker succeeds this time
    vi.mocked(pickNotesFile).mockResolvedValueOnce('/home/user/notes/go.md')
    setupIngestDialogSubscriptions()
    await user.click(screen.getByRole('button', { name: 'Add source' }))

    // Then the stale error is cleared — not just replaced by different text
    await screen.findByText('Importing notes')
    expect(container.querySelector('.text-destructive')).toBeNull()
  })

  it('a cancelled picker does nothing', async () => {
    // Given a picker the user cancelled (resolves to an empty path)
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    vi.mocked(pickNotesFile).mockResolvedValueOnce('')
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')

    // When clicking Add
    await user.click(screen.getByRole('button', { name: 'Add source' }))

    // Then no import dialog opens
    await waitFor(() => expect(pickNotesFile).toHaveBeenCalled())
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
  })

  it('a rejected picker shows the inline error copy', async () => {
    // Given a picker that fails to open
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    vi.mocked(pickNotesFile).mockRejectedValueOnce(new Error('dialog unavailable'))
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')

    // When clicking Add
    await user.click(screen.getByRole('button', { name: 'Add source' }))

    // Then the existing inline error copy shows, and no dialog opens
    expect(
      await screen.findByText('Failed to open the notes picker. Please try again.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
  })

  it('closing the import dialog reloads the sources list', async () => {
    // Given a picker that resolves to a chosen file, and an import that finishes
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    vi.mocked(pickNotesFile).mockResolvedValueOnce('/home/user/notes/go.md')
    const { emitDone } = setupIngestDialogSubscriptions()
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')
    await user.click(screen.getByRole('button', { name: 'Add source' }))
    await screen.findByText('Importing notes')
    emitDone(EMPTY_SUMMARY)

    // When closing the finished dialog
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    await user.click(await findFooterCloseButton())

    // Then the dialog itself closes, and the panel reloads and shows the
    // newly imported document
    expect(await screen.findByText('Distributed Systems')).toBeInTheDocument()
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
    expect(listSessionSources).toHaveBeenCalledTimes(2)
  })

  it('using the empty state\'s "Add a source" action also opens the picker', async () => {
    // Given an empty session
    vi.mocked(listSessionSources).mockResolvedValueOnce([])
    vi.mocked(pickNotesFile).mockResolvedValueOnce('')
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('No sources imported yet.')

    // When clicking the empty state's own Add action
    await user.click(screen.getByRole('button', { name: 'Add a source' }))

    // Then it opens the same picker
    expect(pickNotesFile).toHaveBeenCalledOnce()
  })

  it('disables Add, with a reason, while the index is retrying', async () => {
    // Given the index is retrying
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" mutationsDisabled />)
    await screen.findByText('Distributed Systems')

    // Then Add is disabled, with the reason available as a tooltip
    const addButton = screen.getByRole('button', { name: 'Add source' })
    expect(addButton).toBeDisabled()
    await user.hover(addButton)
    expect(await screen.findByText('Rebuilding knowledge index…')).toBeInTheDocument()
  })

  it('opens the document viewer when a row is clicked', async () => {
    // Given a loaded source and its document
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce({
      itemId: 'item-1',
      title: 'Distributed Systems',
      path: 'notes/ds.md',
      segments: [{ text: 'The scheduler multiplexes M:N goroutines.', chunkId: 'chunk-1' }],
    })
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')

    // When clicking the row
    await user.click(screen.getByText('Distributed Systems'))

    // Then the viewer opens for that document, not a highlighted passage
    expect(getSessionSourceDocument).toHaveBeenCalledWith('session-1', 'item-1')
    await waitFor(() =>
      expect(
        screen.getByText('The scheduler multiplexes M:N goroutines.', { exact: false }),
      ).toBeInTheDocument(),
    )
    // And the list itself is no longer rendered
    expect(screen.queryByPlaceholderText('Search sources')).not.toBeInTheDocument()
  })

  it('returns to the list when the viewer\'s "Sources" back button is clicked', async () => {
    // Given the viewer open for a document
    vi.mocked(listSessionSources).mockResolvedValueOnce([DISTRIBUTED_SYSTEMS])
    vi.mocked(getSessionSourceDocument).mockResolvedValueOnce({
      itemId: 'item-1',
      title: 'Distributed Systems',
      path: 'notes/ds.md',
      segments: [{ text: 'Body text.', chunkId: 'chunk-1' }],
    })
    const user = userEvent.setup()
    render(<StudySourcesPanel sessionId="session-1" />)
    await screen.findByText('Distributed Systems')
    await user.click(screen.getByText('Distributed Systems'))
    await waitFor(() => expect(screen.getByText('Body text.')).toBeInTheDocument())

    // When clicking the back button
    await user.click(screen.getByRole('button', { name: /sources/i }))

    // Then the list is shown again
    expect(screen.getByPlaceholderText('Search sources')).toBeInTheDocument()
    expect(screen.queryByText('Body text.')).not.toBeInTheDocument()
  })
})
