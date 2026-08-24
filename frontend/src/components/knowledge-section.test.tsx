import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { approveKnowledgeItem, listKnowledgeItems, type KnowledgeItem } from '@/lib/knowledge'
import { importFile, importNotes, pickNotesFile, pickNotesFolder } from '@/lib/ingest'
import { KnowledgeSection } from './knowledge-section'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return { ...original, listKnowledgeItems: vi.fn(), approveKnowledgeItem: vi.fn() }
})

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return {
    ...original,
    pickNotesFolder: vi.fn(),
    importNotes: vi.fn(),
    pickNotesFile: vi.fn(),
    importFile: vi.fn(),
    onIngestProgress: vi.fn(() => vi.fn()),
    onIngestDone: vi.fn(() => vi.fn()),
    onIngestError: vi.fn(() => vi.fn()),
  }
})

// Opens the "Import notes" dropdown and clicks the given item.
async function openImportMenu(user: ReturnType<typeof userEvent.setup>, itemName: string) {
  await user.click(screen.getByRole('button', { name: 'Import notes' }))
  await user.click(await screen.findByRole('menuitem', { name: itemName }))
}

function draftItem(id: string): KnowledgeItem {
  return {
    id,
    topic: 'Go',
    concept: `Concept ${id}`,
    definition: 'Definition.',
    properties: [],
    tradeOffs: [],
    relatedConcepts: [],
    source: 'athena',
    status: 'draft',
    createdAt: '2026-08-18T10:00:00Z',
    updatedAt: '2026-08-18T10:00:00Z',
  }
}

describe('KnowledgeSection', () => {
  it('starts on the Explorer tab, querying all statuses', async () => {
    // Given no draft items
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering the section
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // Then Explorer is the active tab and it queries with no status constraint
    expect(screen.getByRole('tab', { name: 'Explorer' })).toHaveAttribute('aria-selected', 'true')
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', ''))
  })

  it('switches to Review, which forces the draft-only filter', async () => {
    // Given the section rendered on Explorer
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When switching to Review
    await user.click(screen.getByRole('tab', { name: 'Review' }))

    // Then Review is now selected and the underlying query is forced to drafts
    expect(screen.getByRole('tab', { name: 'Review' })).toHaveAttribute('aria-selected', 'true')
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', 'draft'))
  })

  it('marks aria-selected on only the actually active tab, and switching back to Explorer works too', async () => {
    // Given the section rendered on Explorer (the default tab)
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )
    expect(screen.getByRole('tab', { name: 'Explorer' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Review' })).toHaveAttribute('aria-selected', 'false')

    // When switching to Review
    await user.click(screen.getByRole('tab', { name: 'Review' }))

    // Then Review is now selected and Explorer is not
    expect(screen.getByRole('tab', { name: 'Review' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Explorer' })).toHaveAttribute('aria-selected', 'false')

    // When switching back to Explorer
    await user.click(screen.getByRole('tab', { name: 'Explorer' }))

    // Then Explorer is selected again
    expect(screen.getByRole('tab', { name: 'Explorer' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Review' })).toHaveAttribute('aria-selected', 'false')
  })

  it('applies the highlight classes to only the active tab, keeping the shared base classes on both', async () => {
    // Given the section rendered on Explorer (the default tab)
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )
    const explorerTab = screen.getByRole('tab', { name: 'Explorer' })
    const reviewTab = screen.getByRole('tab', { name: 'Review' })

    // Then only Explorer carries the active highlight, and both still share
    // the base tab styling
    expect(explorerTab.className).toContain('bg-secondary')
    expect(explorerTab.className).toContain('rounded-md')
    expect(reviewTab.className).not.toContain('bg-secondary')
    expect(reviewTab.className).toContain('rounded-md')

    // When switching to Review
    await user.click(reviewTab)

    // Then the highlight moves to Review, and the base classes remain on both
    expect(reviewTab.className).toContain('bg-secondary')
    expect(reviewTab.className).toContain('rounded-md')
    expect(explorerTab.className).not.toContain('bg-secondary')
    expect(explorerTab.className).toContain('rounded-md')
  })

  it('shows the pending-review count on the Review tab, from the draftCount prop', async () => {
    // Given a draftCount from the parent (AppShell), not fetched locally
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering the section
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={2}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // Then the Review tab carries a badge with that count
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('shows no badge when draftCount is zero', () => {
    // Given a zero draftCount
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering the section
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // Then the Review tab carries no count badge
    expect(
      screen.getByRole('tab', { name: 'Review' }).querySelector('[data-slot="badge"]'),
    ).toBeNull()
  })

  it('threads onKnowledgeChanged into the Review tab, firing it after approving a draft', async () => {
    // Given a single draft item under the Review tab
    const item = draftItem('1')
    vi.mocked(listKnowledgeItems).mockResolvedValue([item])
    vi.mocked(approveKnowledgeItem).mockResolvedValue({ ...item, status: 'approved' })
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={1}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )
    await user.click(screen.getByRole('tab', { name: /Review/ }))
    await user.click(await screen.findByText('Concept 1'))

    // When approving it from inside the Review tab
    await user.click(screen.getByRole('button', { name: 'Approve' }))

    // Then AppShell's badge-freshness callback fires
    await waitFor(() => expect(onKnowledgeChanged).toHaveBeenCalledTimes(1))
  })

  it('offers "Import folder..." and "Import file..." from the Import notes dropdown', async () => {
    // Given the section rendered
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When opening the dropdown
    await user.click(screen.getByRole('button', { name: 'Import notes' }))

    // Then both menu actions are present
    expect(await screen.findByRole('menuitem', { name: 'Import folder...' })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Import file...' })).toBeInTheDocument()
  })

  it('opens the progress dialog and starts the import once a folder is picked', async () => {
    // Given a folder picker that resolves to a chosen path
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockResolvedValueOnce('/home/user/notes')
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When picking "Import folder..." from the dropdown
    await openImportMenu(user, 'Import folder...')

    // Then the progress dialog opens and the import starts for that path
    expect(await screen.findByText('Importing notes')).toBeInTheDocument()
    await waitFor(() => expect(importNotes).toHaveBeenCalledWith('/home/user/notes'))
  })

  it('opens the progress dialog and starts the import once a single file is picked', async () => {
    // Given a file picker that resolves to a chosen path
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFile).mockResolvedValueOnce('/home/user/notes/go.md')
    vi.mocked(importFile).mockReturnValueOnce(new Promise<void>(() => {}))
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When picking "Import file..." from the dropdown
    await openImportMenu(user, 'Import file...')

    // Then the progress dialog opens and the import starts for that file
    expect(await screen.findByText('Importing notes')).toBeInTheDocument()
    expect(screen.getByText('Processing the selected file.')).toBeInTheDocument()
    await waitFor(() => expect(importFile).toHaveBeenCalledWith('/home/user/notes/go.md'))
  })

  it('closes the import dialog when Close is clicked after the import fails', async () => {
    // Given an import that fails outright
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockResolvedValueOnce('/home/user/notes')
    const failedImport = Promise.reject(new Error('IPC failure'))
    failedImport.catch(() => {}) // avoid an unhandled-rejection warning from this local reference
    vi.mocked(importNotes).mockReturnValueOnce(failedImport)
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )
    await openImportMenu(user, 'Import folder...')
    const [closeButton] = await screen.findAllByRole('button', { name: 'Close' })

    // When closing it
    await user.click(closeButton)

    // Then the dialog is gone
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
  })

  it('does nothing when the folder picker is cancelled', async () => {
    // Given a folder picker that returns an empty path (cancelled)
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockResolvedValueOnce('')
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When picking "Import folder..." from the dropdown
    await openImportMenu(user, 'Import folder...')

    // Then no progress dialog opens and no import starts
    await waitFor(() => expect(pickNotesFolder).toHaveBeenCalled())
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
    expect(importNotes).not.toHaveBeenCalled()
  })

  it('does nothing when the file picker is cancelled', async () => {
    // Given a file picker that returns an empty path (cancelled)
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFile).mockResolvedValueOnce('')
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When picking "Import file..." from the dropdown
    await openImportMenu(user, 'Import file...')

    // Then no progress dialog opens and no import starts
    await waitFor(() => expect(pickNotesFile).toHaveBeenCalled())
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
    expect(importFile).not.toHaveBeenCalled()
  })

  it('shows an inline error when the folder picker rejects', async () => {
    // Given a folder picker that rejects
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockRejectedValueOnce(new Error('dialog unavailable'))
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When picking "Import folder..." from the dropdown
    await openImportMenu(user, 'Import folder...')

    // Then the shared inline error is shown, and no import dialog opens
    expect(
      await screen.findByText('Failed to open the notes picker. Please try again.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
  })

  it('shows an inline error when the file picker rejects', async () => {
    // Given a file picker that rejects
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFile).mockRejectedValueOnce(new Error('dialog unavailable'))
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // When picking "Import file..." from the dropdown
    await openImportMenu(user, 'Import file...')

    // Then the shared inline error is shown, and no import dialog opens
    expect(
      await screen.findByText('Failed to open the notes picker. Please try again.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
  })

  it('clears a previous picker error when starting either picker again', async () => {
    // Given a prior picker failure already shown
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockRejectedValueOnce(new Error('dialog unavailable'))
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )
    await openImportMenu(user, 'Import folder...')
    expect(
      await screen.findByText('Failed to open the notes picker. Please try again.'),
    ).toBeInTheDocument()

    // When starting the file picker, which resolves cleanly this time
    vi.mocked(pickNotesFile).mockResolvedValueOnce('')
    await openImportMenu(user, 'Import file...')

    // Then the stale error is cleared
    await waitFor(() => expect(pickNotesFile).toHaveBeenCalled())
    expect(
      screen.queryByText('Failed to open the notes picker. Please try again.'),
    ).not.toBeInTheDocument()
  })

  it('disables both menu items while mutations are disabled', () => {
    // Given the section rendered with mutations disabled
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // Then the dropdown trigger itself is disabled
    expect(screen.getByRole('button', { name: 'Import notes' })).toBeDisabled()
  })
})
