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
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)

    // Then both appear under the same "Go" group, each with its own source badge
    await screen.findByText('Channels')
    expect(screen.getByText('Generics')).toBeInTheDocument()
    expect(screen.getByText('Go')).toBeInTheDocument()
    expect(screen.getByText('Athena')).toBeInTheDocument()
    expect(screen.getByText('Nota importada')).toBeInTheDocument()
  })

  it('requests the current topic and status filter from the backend', async () => {
    // Given items list backed by the topic/status filter
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering the Explorer for a specific topic
    render(<KnowledgeExplorerScreen selectedTopic="Kubernetes" mode="explorer" />)

    // Then it fetches exactly that topic with no status constraint (empty = all)
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('Kubernetes', ''))
  })

  it('forces the status filter to draft and hides the picker in review mode', async () => {
    // Given the review-mode Explorer
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValue([])

    // When rendering it
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="review" />)

    // Then it always queries status=draft and offers no status dropdown
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('', 'draft'))
    expect(screen.queryByText('Status:')).not.toBeInTheDocument()
  })

  it('refetches items when a notes import finishes', async () => {
    // Given a mounted Explorer
    let doneHandler: () => void = () => {}
    vi.mocked(onIngestDone).mockImplementation((handler) => {
      doneHandler = handler as () => void
      return vi.fn()
    })
    vi.mocked(listKnowledgeItems).mockResolvedValue([])
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)
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
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)
    await user.click(await screen.findByText('Channels'))

    // Then Approve is offered, Deprecate is not
    expect(screen.getByRole('button', { name: 'Aprovar' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Descontinuar' })).not.toBeInTheDocument()

    // When approving
    await user.click(screen.getByRole('button', { name: 'Aprovar' }))

    // Then the badge flips to approved without a second list fetch, and
    // Approve is no longer offered on the now-approved item
    await waitFor(() => expect(screen.getAllByText('Aprovado').length).toBeGreaterThan(0))
    expect(listKnowledgeItems).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: 'Aprovar' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Descontinuar' })).toBeInTheDocument()
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
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)
    await user.click(await screen.findByText('Channels'))

    // When deprecating it
    await user.click(screen.getByRole('button', { name: 'Descontinuar' }))

    // Then it becomes deprecated, and neither Approve nor Deprecate remain offered
    await waitFor(() => expect(screen.getAllByText('Descontinuado').length).toBeGreaterThan(0))
    expect(screen.queryByRole('button', { name: 'Aprovar' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Descontinuar' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument()
  })

  it('edits a field and saves without touching status, source or createdAt', async () => {
    // Given a selected approved item
    stubIngestDone()
    const item = testItem({ status: 'approved', source: 'athena' })
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    vi.mocked(updateKnowledgeItem).mockResolvedValueOnce({ ...item, concept: 'Buffered channels' })
    const user = userEvent.setup()
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)
    await user.click(await screen.findByText('Channels'))

    // When editing the concept and saving
    await user.click(screen.getByRole('button', { name: 'Editar' }))
    const conceptInput = screen.getByLabelText('Conceito')
    await user.clear(conceptInput)
    await user.type(conceptInput, 'Buffered channels')
    await user.click(screen.getByRole('button', { name: 'Salvar' }))

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

  it('asks for confirmation before deleting, and removes the item on confirm', async () => {
    // Given a selected item
    stubIngestDone()
    const item = testItem()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([item])
    vi.mocked(deleteKnowledgeItem).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)
    await user.click(await screen.findByText('Channels'))

    // When clicking Excluir
    await user.click(screen.getByRole('button', { name: 'Excluir' }))

    // Then a confirmation dialog appears before anything is deleted
    expect(screen.getByText('Excluir "Channels"?')).toBeInTheDocument()
    expect(deleteKnowledgeItem).not.toHaveBeenCalled()

    // When confirming
    const dialog = screen.getByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Excluir' }))

    // Then the item is deleted and removed from the list
    await waitFor(() => expect(deleteKnowledgeItem).toHaveBeenCalledWith('item-1'))
    await waitFor(() => expect(screen.queryByText('Channels')).not.toBeInTheDocument())
  })

  it('shows only items under the selected topic when a topic is chosen', async () => {
    // Given the backend is queried for a single topic
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([testItem({ topic: 'Kubernetes' })])

    // When rendering with that topic selected
    render(<KnowledgeExplorerScreen selectedTopic="Kubernetes" mode="explorer" />)

    // Then the request carries that topic constraint
    await waitFor(() => expect(listKnowledgeItems).toHaveBeenCalledWith('Kubernetes', ''))
  })

  it('shows an empty state when there are no items', async () => {
    // Given no items at all
    stubIngestDone()
    vi.mocked(listKnowledgeItems).mockResolvedValueOnce([])

    // When rendering the Explorer
    render(<KnowledgeExplorerScreen selectedTopic={null} mode="explorer" />)

    // Then the empty state is shown
    expect(await screen.findByText('Nenhum item encontrado.')).toBeInTheDocument()
  })
})
