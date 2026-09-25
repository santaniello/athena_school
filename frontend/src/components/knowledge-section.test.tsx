import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  approveKnowledgeItem,
  deleteKnowledgeItem,
  listKnowledgeItems,
  listPendingReconciliations,
  type KnowledgeItem,
  type PendingReconciliation,
} from '@/lib/knowledge'
import { KnowledgeSection } from './knowledge-section'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return {
    ...original,
    listKnowledgeItems: vi.fn(),
    approveKnowledgeItem: vi.fn(),
    deleteKnowledgeItem: vi.fn(),
    // Defaults to empty so every existing test below — none of which cares
    // about pending reconciliation proposals — can switch to the Review tab
    // without also needing to stub this out itself.
    listPendingReconciliations: vi.fn().mockResolvedValue([]),
  }
})

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return {
    ...original,
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

function pendingProposal(): PendingReconciliation {
  return {
    id: 'proposal-1',
    action: 'update',
    candidate: draftItem('candidate-1'),
    targetItemId: 'item-target',
    targetConcept: 'Eventual consistency',
    targetStatus: 'approved',
    reason: 'extends the existing definition',
    changes: { properties: [], tradeOffs: [], relatedConcepts: [] },
    stale: false,
    createdAt: '2026-08-28T09:00:00Z',
  }
}

describe('KnowledgeSection', () => {
  it('shows the pending reconciliation queue only on the Review tab', async () => {
    // Given a pending proposal
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    vi.mocked(listPendingReconciliations).mockResolvedValueOnce([pendingProposal()])
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

    // Then the pending reconciliation queue appears
    expect(await screen.findByText('Pending reconciliation')).toBeInTheDocument()

    // When switching back to Explorer
    await user.click(screen.getByRole('tab', { name: 'Explorer' }))

    // Then it is gone — unmounted, not merely hidden, so a stale fetch from
    // its first mount cannot leave it visible regardless of the active tab
    expect(screen.queryByText('Pending reconciliation')).not.toBeInTheDocument()
  })

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

  it('threads onTopicsChanged into the Explorer tab, firing it after deleting an item', async () => {
    // Given a single item on the Explorer tab
    const item = draftItem('1')
    vi.mocked(listKnowledgeItems).mockResolvedValue([item])
    vi.mocked(deleteKnowledgeItem).mockResolvedValue(undefined)
    const onTopicsChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={1}
        onKnowledgeChanged={vi.fn()}
        onTopicsChanged={onTopicsChanged}
      />,
    )
    await user.click(await screen.findByText('Concept 1'))

    // When deleting it
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))

    // Then AppShell's topic-refresh callback fires — deleting an item can
    // remove the last one under its topic
    await waitFor(() => expect(onTopicsChanged).toHaveBeenCalledTimes(1))
  })

  it('offers no global import action — importing belongs to a study session', () => {
    // Given the section rendered with mutations enabled
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When it mounts
    render(
      <KnowledgeSection
        selectedTopic={null}
        mutationsDisabled={false}
        draftCount={0}
        onKnowledgeChanged={vi.fn()}
      />,
    )

    // Then no "Import notes" button is offered: knowledge is now owned by a
    // session, so importing is not a global action
    expect(screen.queryByRole('button', { name: 'Import notes' })).not.toBeInTheDocument()
  })
})
