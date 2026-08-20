import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { listKnowledgeItems, type KnowledgeItem } from '@/lib/knowledge'
import { importNotes, pickNotesFolder } from '@/lib/ingest'
import { KnowledgeSection } from './knowledge-section'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return { ...original, listKnowledgeItems: vi.fn() }
})

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return {
    ...original,
    pickNotesFolder: vi.fn(),
    importNotes: vi.fn(),
    onIngestProgress: vi.fn(() => vi.fn()),
    onIngestDone: vi.fn(() => vi.fn()),
    onIngestError: vi.fn(() => vi.fn()),
  }
})

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
    render(<KnowledgeSection selectedTopic={null} mutationsDisabled={false} />)

    // Then Explorer is the active tab and it queries with no status constraint
    expect(screen.getByRole('tab', { name: 'Explorer' })).toHaveAttribute('aria-selected', 'true')
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', ''))
  })

  it('switches to Review, which forces the draft-only filter', async () => {
    // Given the section rendered on Explorer
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const user = userEvent.setup()
    render(<KnowledgeSection selectedTopic={null} mutationsDisabled={false} />)

    // When switching to Review
    await user.click(screen.getByRole('tab', { name: 'Review' }))

    // Then Review is now selected and the underlying query is forced to drafts
    expect(screen.getByRole('tab', { name: 'Review' })).toHaveAttribute('aria-selected', 'true')
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', 'draft'))
  })

  it('shows the pending-review count on the Review tab', async () => {
    // Given two draft items — mocked by status, since the section's own
    // draft-count fetch and the nested Explorer's own fetch (status: '')
    // both call listKnowledgeItems independently
    vi.mocked(listKnowledgeItems).mockImplementation((_topic, status) =>
      Promise.resolve(status === 'draft' ? [draftItem('1'), draftItem('2')] : []),
    )

    // When rendering the section
    render(<KnowledgeSection selectedTopic={null} mutationsDisabled={false} />)

    // Then the Review tab carries a badge with that count
    expect(await screen.findByText('2')).toBeInTheDocument()
  })

  it('shows no badge when there are no drafts pending review', async () => {
    // Given no draft items
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering the section
    render(<KnowledgeSection selectedTopic={null} mutationsDisabled={false} />)

    // Then the Review tab carries no count badge
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalled())
    expect(
      screen.getByRole('tab', { name: 'Review' }).querySelector('[data-slot="badge"]'),
    ).toBeNull()
  })

  it('opens the progress dialog and starts the import once a folder is picked', async () => {
    // Given a folder picker that resolves to a chosen path
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockResolvedValueOnce('/home/user/notes')
    vi.mocked(importNotes).mockReturnValueOnce(new Promise<void>(() => {}))
    const user = userEvent.setup()
    render(<KnowledgeSection selectedTopic={null} mutationsDisabled={false} />)

    // When clicking "Import notes"
    await user.click(screen.getByRole('button', { name: 'Import notes' }))

    // Then the progress dialog opens and the import starts for that path
    expect(await screen.findByText('Importing notes')).toBeInTheDocument()
    await waitFor(() => expect(importNotes).toHaveBeenCalledWith('/home/user/notes'))
  })

  it('does nothing when the folder picker is cancelled', async () => {
    // Given a folder picker that returns an empty path (cancelled)
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(pickNotesFolder).mockResolvedValueOnce('')
    const user = userEvent.setup()
    render(<KnowledgeSection selectedTopic={null} mutationsDisabled={false} />)

    // When clicking "Import notes"
    await user.click(screen.getByRole('button', { name: 'Import notes' }))

    // Then no progress dialog opens and no import starts
    await waitFor(() => expect(pickNotesFolder).toHaveBeenCalled())
    expect(screen.queryByText('Importing notes')).not.toBeInTheDocument()
    expect(importNotes).not.toHaveBeenCalled()
  })
})
