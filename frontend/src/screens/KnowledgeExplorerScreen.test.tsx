import { describe, expect, it, vi } from 'vitest'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  approveKnowledgeItem,
  deleteKnowledgeItem,
  deprecateKnowledgeItem,
  listKnowledgeItems,
  updateKnowledgeItem,
  type KnowledgeItem,
} from '@/lib/knowledge'
import { onIngestDone } from '@/lib/ingest'
import KnowledgeExplorerScreen from './KnowledgeExplorerScreen'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return {
    ...original,
    listKnowledgeItems: vi.fn(),
    approveKnowledgeItem: vi.fn(),
    deprecateKnowledgeItem: vi.fn(),
    updateKnowledgeItem: vi.fn(),
    deleteKnowledgeItem: vi.fn(),
  }
})

vi.mock('@/lib/ingest', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/ingest')>()
  return { ...original, onIngestDone: vi.fn() }
})

function testItem(overrides: Partial<KnowledgeItem> = {}): KnowledgeItem {
  return {
    id: 'item-1',
    topic: 'Go',
    concept: 'Channels',
    definition: 'Typed conduits for goroutine communication.',
    properties: ['typed'],
    tradeOffs: ['coordination overhead'],
    relatedConcepts: ['goroutines'],
    source: 'athena',
    status: 'draft',
    createdAt: '2026-08-18T10:00:00Z',
    updatedAt: '2026-08-18T10:00:00Z',
    ...overrides,
  }
}

function stubIngestDone() {
  const unsubscribe = vi.fn()
  vi.mocked(onIngestDone).mockReturnValue(unsubscribe)
  return unsubscribe
}

describe('KnowledgeExplorerScreen', () => {
  it('lists items from both sources under the same topic group, distinguished by a source badge', async () => {
    // Given one Athena item and one imported note, both under "Go"
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([
      testItem({ id: 'a', concept: 'Channels', source: 'athena' }),
      testItem({ id: 'b', concept: 'Generics', source: 'imported_doc', status: 'approved' }),
    ])

    // When rendering the Explorer for all topics
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )

    // Then both appear under the same "Go" group, each with its own source badge
    await screen.findByText('Channels')
    expect(screen.getByText('Generics')).toBeInTheDocument()
    expect(screen.getByText('Go')).toBeInTheDocument()
    expect(screen.getByText('Athena')).toBeInTheDocument()
    expect(screen.getByText('Imported note')).toBeInTheDocument()

    // And no error banner shows for a plain, successful load
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('labels a user-authored note with its own "User note" source badge', async () => {
    // Given a single item authored directly by the user (not Athena, not imported)
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([testItem({ source: 'user_note' })])

    // When rendering the Explorer
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )

    // Then its source badge reads "User note"
    await screen.findByText('Channels')
    expect(screen.getByText('User note')).toBeInTheDocument()
  })

  it('renders each property/trade-off/related-concept as its own list item, and omits the section entirely when empty', async () => {
    // Given a selected item with two properties but no trade-offs
    stubIngestDone()
    const item = testItem({
      properties: ['typed', 'buffered'],
      tradeOffs: [],
      relatedConcepts: ['goroutines'],
    })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // Then every property renders as its own item under its own heading
    expect(screen.getByText('Properties')).toBeInTheDocument()
    expect(screen.getByText('typed')).toBeInTheDocument()
    expect(screen.getByText('buffered')).toBeInTheDocument()
    expect(screen.getByText('Related concepts')).toBeInTheDocument()
    expect(screen.getByText('goroutines')).toBeInTheDocument()

    // And the empty trade-offs section is skipped entirely, not shown empty
    expect(screen.queryByText('Trade-offs')).not.toBeInTheDocument()
  })

  it('requests the current topic and status filter from the backend', async () => {
    // Given items list backed by the topic/status filter
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering the Explorer for a specific topic
    render(
      <KnowledgeExplorerScreen
        selectedTopic="Kubernetes"
        mode="explorer"
        mutationsDisabled={false}
      />,
    )

    // Then it fetches exactly that topic with no status constraint (empty = all)
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('Kubernetes', ''))
  })

  it('ignores a stale response from a previous filter after the topic changes', async () => {
    // Given a slow request for the initial topic that has not resolved yet
    stubIngestDone()
    let resolveGo: (items: KnowledgeItem[]) => void = () => {}
    const goPromise = new Promise<KnowledgeItem[]>((resolve) => {
      resolveGo = resolve
    })
    vi.mocked(listKnowledgeItems).mockReturnValueOnce(goPromise)
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([
      testItem({ id: 'k', concept: 'Pods', topic: 'Kubernetes' }),
    ])
    const { rerender } = render(
      <KnowledgeExplorerScreen selectedTopic="Go" mode="explorer" mutationsDisabled={false} />,
    )
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('Go', ''))

    // When the topic changes before that first request resolves, and the
    // new request settles first
    rerender(
      <KnowledgeExplorerScreen
        selectedTopic="Kubernetes"
        mode="explorer"
        mutationsDisabled={false}
      />,
    )
    await screen.findByText('Pods')

    // Then the stale "Go" response arriving afterward is ignored instead
    // of overwriting the current "Kubernetes" filter's items
    await act(async () => {
      resolveGo([testItem({ id: 'g', concept: 'Channels', topic: 'Go' })])
    })
    expect(screen.queryByText('Channels')).not.toBeInTheDocument()
    expect(screen.getByText('Pods')).toBeInTheDocument()
  })

  it('ignores a stale response from the initial load when ingest:done triggers a second load first', async () => {
    // Given the initial load still pending
    let doneHandler: () => void = () => {}
    vi.mocked(onIngestDone).mockImplementation((handler) => {
      doneHandler = handler as () => void
      return vi.fn()
    })
    let resolveInitial: (items: KnowledgeItem[]) => void = () => {}
    const initialLoad = new Promise<KnowledgeItem[]>((resolve) => {
      resolveInitial = resolve
    })
    vi.mocked(listKnowledgeItems).mockReturnValueOnce(initialLoad)
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([
      testItem({ id: 'r', concept: 'Refetched' }),
    ])
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledTimes(1))

    // When a notes import completes before the initial load resolves,
    // starting a second load that settles first
    await act(() => doneHandler())
    await screen.findByText('Refetched')

    // Then the stale initial response arriving afterward is ignored,
    // instead of overwriting the newer refetched items
    await act(async () => {
      resolveInitial([testItem({ id: 'i', concept: 'Initial' })])
    })
    expect(screen.queryByText('Initial')).not.toBeInTheDocument()
    expect(screen.getByText('Refetched')).toBeInTheDocument()
  })

  it('unsubscribes the previous ingest:done listener when the topic changes', async () => {
    // Given a mounted Explorer for one topic
    const unsubscribe = stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const { rerender } = render(
      <KnowledgeExplorerScreen selectedTopic="Go" mode="explorer" mutationsDisabled={false} />,
    )
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('Go', ''))
    expect(unsubscribe).not.toHaveBeenCalled()

    // When the topic changes
    rerender(
      <KnowledgeExplorerScreen
        selectedTopic="Kubernetes"
        mode="explorer"
        mutationsDisabled={false}
      />,
    )

    // Then the previous topic's ingest:done subscription is torn down
    expect(unsubscribe).toHaveBeenCalled()
  })

  it('ignores a stale rejection from the initial load when ingest:done triggers a second load first, without surfacing its error', async () => {
    // Given the initial load still pending
    let doneHandler: () => void = () => {}
    vi.mocked(onIngestDone).mockImplementation((handler) => {
      doneHandler = handler as () => void
      return vi.fn()
    })
    let rejectInitial: (error: Error) => void = () => {}
    const initialLoad = new Promise<KnowledgeItem[]>((_resolve, reject) => {
      rejectInitial = reject
    })
    initialLoad.catch(() => {}) // avoid an unhandled-rejection warning from this local reference
    vi.mocked(listKnowledgeItems).mockReturnValueOnce(initialLoad)
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([
      testItem({ id: 'r', concept: 'Refetched' }),
    ])
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledTimes(1))

    // When a notes import completes before the initial load rejects,
    // starting a second load that settles successfully first
    await act(() => doneHandler())
    await screen.findByText('Refetched')

    // Then the stale initial rejection arriving afterward is ignored,
    // instead of surfacing a loading error over the now-current items
    await act(async () => {
      rejectInitial(new Error('stale failure'))
    })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByText('Refetched')).toBeInTheDocument()
  })

  it('reflects the selected status in the dropdown, and maps "All" back to no backend constraint', async () => {
    // Given the Explorer mounted with no status filter
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    const trigger = await screen.findByRole('combobox', { name: 'Status:' })
    expect(within(trigger).getByText('All')).toBeInTheDocument()
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', ''))

    // When selecting Draft
    await user.click(trigger)
    await user.click(await screen.findByRole('option', { name: 'Draft' }))

    // Then the trigger reflects it, and the backend query carries that status
    expect(within(trigger).getByText('Draft')).toBeInTheDocument()
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', 'draft'))

    // When switching back to All
    await user.click(trigger)
    await user.click(await screen.findByRole('option', { name: 'All' }))

    // Then the trigger reflects that too, and the query drops the constraint
    expect(within(trigger).getByText('All')).toBeInTheDocument()
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenLastCalledWith('', ''))
  })

  it('drops an item from the list once a status change during a pending approval leaves it filtered out, matching the filter current now rather than at mount', async () => {
    // Given a draft item, with the Explorer's status filter switched to Draft
    // (a real change away from the mount-time default, unlike mode="review"
    // where the filter never changes)
    stubIngestDone()
    const draftItem = testItem({ status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValue([draftItem])
    let resolveApprove: (item: KnowledgeItem) => void = () => {}
    vi.mocked(approveKnowledgeItem).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveApprove = resolve
      }),
    )
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByRole('combobox', { name: 'Status:' }))
    await user.click(await screen.findByRole('option', { name: 'Draft' }))
    await user.click(await screen.findByText('Channels'))

    // When approving it (left pending) without changing the filter again
    await user.click(screen.getByRole('button', { name: 'Approve' }))
    await act(async () => {
      resolveApprove({ ...draftItem, status: 'approved' })
    })

    // Then it drops out of the Draft-filtered list — the filter current
    // *now* (Draft) is respected, not the filter that happened to be
    // active when the component first mounted
    expect(screen.queryByText('Channels')).not.toBeInTheDocument()
  })

  it('checks the filter current when a mutation resolves, not the one captured when it started', async () => {
    // Given a draft item, with the status filter set to Draft
    stubIngestDone()
    const draftItem = testItem({ status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValue([draftItem])
    let resolveApprove: (item: KnowledgeItem) => void = () => {}
    vi.mocked(approveKnowledgeItem).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveApprove = resolve
      }),
    )
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByRole('combobox', { name: 'Status:' }))
    await user.click(await screen.findByRole('option', { name: 'Draft' }))
    await user.click(await screen.findByText('Channels'))

    // When approving it (left pending), then the filter is switched to
    // "All" before that approval resolves
    await user.click(screen.getByRole('button', { name: 'Approve' }))
    await user.click(screen.getByRole('combobox', { name: 'Status:' }))
    await user.click(await screen.findByRole('option', { name: 'All' }))
    await act(async () => {
      resolveApprove({ ...draftItem, status: 'approved' })
    })

    // Then the item stays visible under the current "All" filter instead
    // of being dropped for no longer matching the "Draft" filter that was
    // active only when the approval started
    expect(screen.getAllByText('Channels').length).toBeGreaterThan(0)
  })

  it('highlights only the selected item in the list, keeping the shared row styling on both', async () => {
    // Given two items in the list
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([
      testItem({ id: 'a', concept: 'Channels' }),
      testItem({ id: 'b', concept: 'Generics' }),
    ])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await screen.findByText('Channels')
    const channelsButton = screen.getByText('Channels').closest('button') as HTMLElement
    const genericsButton = screen.getByText('Generics').closest('button') as HTMLElement

    // Then neither is highlighted yet, though both share the base row class
    expect(channelsButton.className).not.toContain('bg-secondary')
    expect(channelsButton.className).toContain('rounded-lg')
    expect(genericsButton.className).not.toContain('bg-secondary')
    expect(genericsButton.className).toContain('rounded-lg')

    // When selecting one item
    await user.click(channelsButton)

    // Then only that item is highlighted, and both keep the base row class
    expect(channelsButton.className).toContain('bg-secondary')
    expect(channelsButton.className).toContain('rounded-lg')
    expect(genericsButton.className).not.toContain('bg-secondary')
    expect(genericsButton.className).toContain('rounded-lg')
  })

  it('styles the approved status badge distinctly from draft/deprecated, in both the list and the detail pane', async () => {
    // Given a draft and an approved item
    stubIngestDone()
    const draftItem = testItem({ id: 'd', concept: 'Draft concept', status: 'draft' })
    const approvedItem = testItem({ id: 'a', concept: 'Approved concept', status: 'approved' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([draftItem, approvedItem])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await screen.findByText('Draft concept')

    // Then the list badges style the approved item distinctly from the draft one
    expect(screen.getByText('Draft').className).toContain('bg-muted')
    expect(screen.getByText('Approved').className).toContain('bg-primary')

    // When selecting the draft item, its detail badge is styled muted too
    await user.click(screen.getByText('Draft concept'))
    for (const badge of screen.getAllByText('Draft')) {
      expect(badge.className).toContain('bg-muted')
    }

    // When selecting the approved item instead, its detail badge is styled
    // as the default/primary variant
    await user.click(screen.getByText('Approved concept'))
    for (const badge of screen.getAllByText('Approved')) {
      expect(badge.className).toContain('bg-primary')
    }
  })

  it('shows a generic error when a mutation action fails, and clears it when selecting a different item', async () => {
    // Given two draft items
    stubIngestDone()
    const itemA = testItem({ id: 'a', concept: 'Channels', status: 'draft' })
    const itemB = testItem({ id: 'b', concept: 'Generics', status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([itemA, itemB])
    vi.mocked(approveKnowledgeItem).mockRejectedValueOnce(new Error('backend rejected it'))
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // When approving it fails
    await user.click(screen.getByRole('button', { name: 'Approve' }))

    // Then a generic error is shown, not a raw backend message
    expect(await screen.findByText('An error occurred. Please try again.')).toBeInTheDocument()

    // When selecting a different item
    await user.click(screen.getByText('Generics'))

    // Then the stale error clears instead of lingering next to it
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('leaves the edit form when selecting a different item, discarding the in-progress edit', async () => {
    // Given a draft item with its edit form open
    stubIngestDone()
    const itemA = testItem({ id: 'a', concept: 'Channels', status: 'draft' })
    const itemB = testItem({ id: 'b', concept: 'Generics', status: 'approved' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([itemA, itemB])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByLabelText('Topic')).toBeInTheDocument()

    // When selecting a different item
    await user.click(screen.getByText('Generics'))

    // Then the edit form is gone, replaced by that other item's detail view
    expect(screen.queryByLabelText('Topic')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Generics' })).toBeInTheDocument()
  })

  it('forces the status filter to draft and hides the picker in review mode', async () => {
    // Given the review-mode Explorer
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering it
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="review" mutationsDisabled={false} />)

    // Then it always queries status=draft and offers no status dropdown
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', 'draft'))
    expect(screen.queryByText('Status:')).not.toBeInTheDocument()
  })

  it('clears the loading error once a later reload succeeds', async () => {
    // Given an Explorer whose initial load failed
    let doneHandler: () => void = () => {}
    vi.mocked(onIngestDone).mockImplementation((handler) => {
      doneHandler = handler as () => void
      return vi.fn()
    })
    vi.mocked(listKnowledgeItems).mockRejectedValueOnce(new Error('offline'))
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([testItem()])
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    expect(await screen.findByText('Failed to load knowledge items.')).toBeInTheDocument()

    // When a notes import completes and the reload succeeds
    await act(() => doneHandler())

    // Then the stale loading error is cleared instead of lingering next
    // to the now-current items
    await waitFor(() =>
      expect(screen.queryByText('Failed to load knowledge items.')).not.toBeInTheDocument(),
    )
    expect(screen.getByText('Channels')).toBeInTheDocument()
  })

  it('refetches items when a notes import finishes', async () => {
    // Given a mounted Explorer
    let doneHandler: () => void = () => {}
    vi.mocked(onIngestDone).mockImplementation((handler) => {
      doneHandler = handler as () => void
      return vi.fn()
    })
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledTimes(1))

    // When an import completes
    await act(() => doneHandler())

    // Then the list is refetched
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledTimes(2))
  })

  it('offers Approve only on a draft item, and applies it without a full refetch', async () => {
    // Given a draft item, selected
    stubIngestDone()
    const draftItem = testItem({ status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([draftItem])
    vi.mocked(approveKnowledgeItem).mockResolvedValueOnce({ ...draftItem, status: 'approved' })
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // Then Approve is offered, Deprecate is not
    expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Deprecate' })).not.toBeInTheDocument()

    // When approving
    await user.click(screen.getByRole('button', { name: 'Approve' }))

    // Then the badge flips to approved without a second list fetch, and
    // Approve is no longer offered on the now-approved item
    await waitFor(() => expect(screen.getAllByText('Approved').length).toBeGreaterThan(0))
    expect(listKnowledgeItems).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Approve' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Deprecate' })).toBeInTheDocument()
  })

  it('removes an item from the Review queue once approved, since it no longer matches the draft filter', async () => {
    // Given two draft items selected in Review mode (status filter fixed to
    // draft), one of them selected
    stubIngestDone()
    const draftItem = testItem({ id: 'a', concept: 'Channels', status: 'draft' })
    const otherDraft = testItem({ id: 'b', concept: 'Generics', status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([draftItem, otherDraft])
    vi.mocked(approveKnowledgeItem).mockResolvedValueOnce({ ...draftItem, status: 'approved' })
    const user = userEvent.setup()
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="review" mutationsDisabled={false} />)
    await user.click(await screen.findByText('Channels'))

    // When approving it
    await user.click(screen.getByRole('button', { name: 'Approve' }))

    // Then it disappears from the queue instead of lingering there with an
    // approved badge, its detail pane clears since it was selected, and the
    // unrelated other draft item is left untouched in the list
    await waitFor(() => expect(screen.queryByText('Channels')).not.toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Approve' })).not.toBeInTheDocument()
    expect(screen.getByText('Select an item to see the details.')).toBeInTheDocument()
    expect(screen.getByText('Generics')).toBeInTheDocument()
  })

  it('keeps a different, currently selected item selected when an unrelated item drops out of the active filter', async () => {
    // Given two draft items in Review mode (status filter fixed to draft)
    stubIngestDone()
    const itemA = testItem({ id: 'a', concept: 'Channels', status: 'draft' })
    const itemB = testItem({ id: 'b', concept: 'Generics', status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([itemA, itemB])
    let resolveApprove: (item: KnowledgeItem) => void = () => {}
    vi.mocked(approveKnowledgeItem).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveApprove = resolve
      }),
    )
    const user = userEvent.setup()
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="review" mutationsDisabled={false} />)
    await user.click(await screen.findByText('Channels'))

    // When approving item A (left pending), then selecting item B instead
    // before that approval resolves
    await user.click(screen.getByRole('button', { name: 'Approve' }))
    await user.click(screen.getByText('Generics'))
    await act(async () => {
      resolveApprove({ ...itemA, status: 'approved' })
    })

    // Then item A drops out of the Draft-filtered queue, but item B — the
    // item actually selected now — stays selected instead of the detail
    // pane reverting to its placeholder
    await waitFor(() => expect(screen.queryByText('Channels')).not.toBeInTheDocument())
    expect(screen.getByRole('heading', { name: 'Generics' })).toBeInTheDocument()
  })

  it('clears the selection when the open item moves outside the filter, instead of silently keeping a stale id that could resurface it later', async () => {
    // Given one draft item, with the Explorer's status filter switched to
    // Draft (a real change away from the mount-time default)
    stubIngestDone()
    const item = testItem({ status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValue([item])
    vi.mocked(approveKnowledgeItem).mockResolvedValueOnce({ ...item, status: 'approved' })
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByRole('combobox', { name: 'Status:' }))
    await user.click(await screen.findByRole('option', { name: 'Draft' }))
    await user.click(await screen.findByText('Channels'))

    // When approving it, dropping it out of the Draft-filtered list
    await user.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() => expect(screen.queryByText('Channels')).not.toBeInTheDocument())

    // And the filter is then switched back to All, bringing the
    // now-approved item back into view
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([{ ...item, status: 'approved' }])
    await user.click(screen.getByRole('combobox', { name: 'Status:' }))
    await user.click(await screen.findByRole('option', { name: 'All' }))
    await screen.findByText('Channels')

    // Then it does not auto-reappear as selected — the selection was
    // properly cleared when it dropped out, not left stuck on its old id
    expect(screen.getByText('Select an item to see the details.')).toBeInTheDocument()
  })

  it('offers Deprecate only on an approved item, including an imported note', async () => {
    // Given an approved, imported-note item
    stubIngestDone()
    const approvedItem = testItem({ status: 'approved', source: 'imported_doc' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([approvedItem])
    vi.mocked(deprecateKnowledgeItem).mockResolvedValueOnce({
      ...approvedItem,
      status: 'deprecated',
    })
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // When deprecating it
    await user.click(screen.getByRole('button', { name: 'Deprecate' }))

    // Then it becomes deprecated, and neither Approve nor Deprecate remain offered
    await waitFor(() => expect(screen.getAllByText('Deprecated').length).toBeGreaterThan(0))
    expect(screen.queryByRole('button', { name: 'Approve' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Deprecate' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
  })

  it('edits a field and saves without touching status, source or createdAt', async () => {
    // Given a selected approved item
    stubIngestDone()
    const item = testItem({ status: 'approved', source: 'athena' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    vi.mocked(updateKnowledgeItem).mockResolvedValueOnce({ ...item, concept: 'Buffered channels' })
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // When editing the concept and saving
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const conceptInput = screen.getByLabelText('Concept')
    await user.clear(conceptInput)
    await user.type(conceptInput, 'Buffered channels')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Then only the editable fields are sent — id/status/source/createdAt
    // are never part of the update call's payload
    await waitFor(() =>
      expect(updateKnowledgeItem).toHaveBeenCalledWith('item-1', {
        topic: 'Go',
        concept: 'Buffered channels',
        definition: item.definition,
        properties: item.properties,
        tradeOffs: item.tradeOffs,
        relatedConcepts: item.relatedConcepts,
      }),
    )
    // And the edited value now shows in the (closed) detail view's heading
    expect(await screen.findByRole('heading', { name: 'Buffered channels' })).toBeInTheDocument()
  })

  it('edits every field, including the tag-list fields, and sends them all', async () => {
    // Given a selected approved item with existing tags
    stubIngestDone()
    const item = testItem({ status: 'approved', source: 'athena' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    vi.mocked(updateKnowledgeItem).mockResolvedValueOnce(item)
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))
    await user.click(screen.getByRole('button', { name: 'Edit' }))

    // When editing the topic and definition text fields
    const topicInput = screen.getByLabelText('Topic')
    await user.clear(topicInput)
    await user.type(topicInput, 'Concurrency')
    const definitionInput = screen.getByLabelText('Definition')
    await user.clear(definitionInput)
    await user.type(definitionInput, 'Updated definition.')

    // And adding a tag to each of the three tag-list fields
    await user.type(screen.getByLabelText('Properties'), 'concurrent-safe{Enter}')
    await user.type(screen.getByLabelText('Trade-offs'), 'GC pressure{Enter}')
    await user.type(screen.getByLabelText('Related concepts'), 'select statement{Enter}')

    // And saving
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Then every edited field reaches the update call, tag lists included
    await waitFor(() =>
      expect(updateKnowledgeItem).toHaveBeenCalledWith('item-1', {
        topic: 'Concurrency',
        concept: item.concept,
        definition: 'Updated definition.',
        properties: [...item.properties, 'concurrent-safe'],
        tradeOffs: [...item.tradeOffs, 'GC pressure'],
        relatedConcepts: [...item.relatedConcepts, 'select statement'],
      }),
    )
  })

  it('discards the in-progress edit when Cancel is clicked, without saving anything', async () => {
    // Given a selected item with its edit form open
    stubIngestDone()
    const item = testItem()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const topicInput = screen.getByLabelText('Topic')
    await user.clear(topicInput)
    await user.type(topicInput, 'Something else entirely')

    // When clicking Cancel
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then the edit form is gone, nothing was saved, and the detail view
    // reverts to the original, unedited item
    expect(screen.queryByLabelText('Topic')).not.toBeInTheDocument()
    expect(updateKnowledgeItem).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: 'Channels' })).toBeInTheDocument()
  })

  it('keeps an edited item in place, without disturbing unrelated items, when it still matches the active status filter', async () => {
    // Given two approved items, one selected
    stubIngestDone()
    const item = testItem({ id: 'a', concept: 'Channels', status: 'approved' })
    const other = testItem({ id: 'b', concept: 'Generics', status: 'approved' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item, other])
    vi.mocked(updateKnowledgeItem).mockResolvedValueOnce({ ...item, concept: 'Buffered channels' })
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // When editing it (an edit that does not change its status) and saving
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const conceptInput = screen.getByLabelText('Concept')
    await user.clear(conceptInput)
    await user.type(conceptInput, 'Buffered channels')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Then the edited item is updated in place in the list — still present,
    // under its new name — and the unrelated item is untouched, not
    // overwritten by the edit
    await screen.findByRole('heading', { name: 'Buffered channels' })
    expect(screen.getAllByText('Buffered channels').length).toBeGreaterThan(0)
    expect(screen.getByText('Generics')).toBeInTheDocument()
  })

  it('asks for confirmation before deleting, and removes the item on confirm', async () => {
    // Given two selected items in the list
    stubIngestDone()
    const item = testItem({ id: 'a', concept: 'Channels' })
    const other = testItem({ id: 'b', concept: 'Generics' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item, other])
    let resolveDelete: () => void = () => {}
    vi.mocked(deleteKnowledgeItem).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveDelete = resolve
      }),
    )
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))

    // When clicking Delete
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    // Then a confirmation dialog appears before anything is deleted
    expect(screen.getByText('Delete "Channels"?')).toBeInTheDocument()
    expect(deleteKnowledgeItem).not.toHaveBeenCalled()

    // When confirming
    const dialog = screen.getByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))

    // Then the dialog closes immediately, before the delete request settles
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()

    // And once it resolves, only the deleted item is gone — the unrelated
    // one stays
    await act(async () => resolveDelete())
    await waitFor(() => expect(deleteKnowledgeItem).toHaveBeenCalledWith('a'))
    await waitFor(() => expect(screen.queryByText('Channels')).not.toBeInTheDocument())
    expect(screen.getByText('Generics')).toBeInTheDocument()
  })

  it('cancels the delete confirmation without deleting anything', async () => {
    // Given a selected item with the delete confirmation open
    stubIngestDone()
    const item = testItem()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')

    // When cancelling
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    // Then the dialog closes and nothing is deleted
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(deleteKnowledgeItem).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: 'Channels' })).toBeInTheDocument()
  })

  it('shows only items under the selected topic when a topic is chosen', async () => {
    // Given the backend is queried for a single topic
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([testItem({ topic: 'Kubernetes' })])

    // When rendering with that topic selected
    render(
      <KnowledgeExplorerScreen
        selectedTopic="Kubernetes"
        mode="explorer"
        mutationsDisabled={false}
      />,
    )

    // Then the request carries that topic constraint
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('Kubernetes', ''))
  })

  it('shows an empty state when there are no items', async () => {
    // Given no items at all
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([])

    // When rendering the Explorer
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )

    // Then the empty state is shown
    expect(await screen.findByText('No items found.')).toBeInTheDocument()
  })

  it('disables every mutation action while mutationsDisabled is true, since the backend guard rejects them during a retry', async () => {
    // Given a selected draft item, rendered while the index is retrying
    stubIngestDone()
    const item = testItem({ status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    const user = userEvent.setup()
    render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={true} />,
    )
    await user.click(await screen.findByText('Channels'))

    // Then Approve, Edit and Delete are all disabled
    expect(screen.getByRole('button', { name: 'Approve' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
  })

  it('disables Save once mutationsDisabled turns true while an edit is already open', async () => {
    // Given an edit already open before the index starts retrying
    stubIngestDone()
    const item = testItem({ status: 'draft' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    const user = userEvent.setup()
    const { rerender } = render(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={false} />,
    )
    await user.click(await screen.findByText('Channels'))
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()

    // When a retry starts and mutationsDisabled flips to true mid-edit
    rerender(
      <KnowledgeExplorerScreen selectedTopic={null} mode="explorer" mutationsDisabled={true} />,
    )

    // Then Save is disabled, since the backend guard would reject it anyway
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })
})
