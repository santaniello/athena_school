import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  acknowledgeReconciliationNoChange,
  applyReconciliationCreate,
  applyReconciliationRelate,
  applyReconciliationUpdate,
  CONFLICT_CREATE_SEPARATELY,
  CONFLICT_KEEP_EXISTING,
  CONFLICT_UPDATE_EXISTING,
  discardExtraction,
  RECONCILE_CONFLICT,
  RECONCILE_CREATE,
  RECONCILE_NO_CHANGE,
  RECONCILE_RELATE,
  RECONCILE_UPDATE,
  resolveReconciliationConflict,
  saveReconciliationForReview,
  type KnowledgeItem,
} from '@/lib/knowledge'
import { KnowledgeExtractionDialog } from './knowledge-extraction-dialog'

vi.mock('@/lib/knowledge', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/knowledge')>()
  return {
    ...original,
    applyReconciliationCreate: vi.fn(),
    applyReconciliationUpdate: vi.fn(),
    applyReconciliationRelate: vi.fn(),
    resolveReconciliationConflict: vi.fn(),
    acknowledgeReconciliationNoChange: vi.fn(),
    saveReconciliationForReview: vi.fn(),
    discardExtraction: vi.fn(),
  }
})

function candidate(id: string, concept: string): KnowledgeItem {
  return {
    id,
    topic: 'Go',
    concept,
    definition: `${concept} definition`,
    properties: [`${concept} property`],
    tradeOffs: [`${concept} trade-off`],
    relatedConcepts: [`${concept} relation`],
    source: 'athena',
    status: 'draft',
    createdAt: '2026-08-18T10:00:00Z',
    updatedAt: '2026-08-18T10:00:00Z',
    duplicates: [],
    semanticCheckUnavailable: false,
    reconciliation: {
      action: RECONCILE_CREATE,
      targetItemId: '',
      reason: 'no existing match found in this topic',
      changes: { properties: [], tradeOffs: [], relatedConcepts: [] },
    },
  }
}

function updateCandidate(id: string, concept: string): KnowledgeItem {
  return {
    ...candidate(id, concept),
    duplicates: [
      {
        itemId: 'existing-1',
        concept: 'Existing ' + concept,
        status: 'approved',
        matchType: 'exact',
        score: 1,
      },
    ],
    reconciliation: {
      action: RECONCILE_UPDATE,
      targetItemId: 'existing-1',
      reason: 'extends the existing definition',
      changes: {
        definition: 'A richer definition.',
        properties: [],
        tradeOffs: [],
        relatedConcepts: [],
      },
    },
  }
}

function relateCandidate(id: string, concept: string): KnowledgeItem {
  return {
    ...candidate(id, concept),
    duplicates: [
      {
        itemId: 'existing-2',
        concept: 'Related ' + concept,
        status: 'approved',
        matchType: 'semantic',
        score: 0.93,
      },
    ],
    reconciliation: {
      action: RECONCILE_RELATE,
      targetItemId: 'existing-2',
      reason: 'distinct but connected concept',
      changes: { properties: [], tradeOffs: [], relatedConcepts: [] },
    },
  }
}

function conflictCandidate(id: string, concept: string): KnowledgeItem {
  return {
    ...candidate(id, concept),
    duplicates: [
      {
        itemId: 'existing-3',
        concept: 'Conflicting ' + concept,
        status: 'approved',
        matchType: 'exact',
        score: 1,
      },
    ],
    reconciliation: {
      action: RECONCILE_CONFLICT,
      targetItemId: 'existing-3',
      reason: 'contradicts the existing definition',
      changes: { properties: [], tradeOffs: [], relatedConcepts: [] },
    },
  }
}

function noChangeCandidate(id: string, concept: string): KnowledgeItem {
  return {
    ...candidate(id, concept),
    duplicates: [
      { itemId: 'existing-4', concept, status: 'approved', matchType: 'exact', score: 1 },
    ],
    reconciliation: {
      action: RECONCILE_NO_CHANGE,
      targetItemId: 'existing-4',
      reason: 'already captured',
      changes: { properties: [], tradeOffs: [], relatedConcepts: [] },
    },
  }
}

describe('KnowledgeExtractionDialog', () => {
  it('lists concept and definition with every batch-zone candidate checked', () => {
    // Given two plain create candidates
    const items = [candidate('1', 'Channels'), candidate('2', 'Goroutines')]

    // When opening the dialog
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // Then concepts and definitions are visible and all candidates are selected
    expect(screen.getByText('Channels')).toBeInTheDocument()
    expect(screen.getByText('Channels definition')).toBeInTheDocument()
    expect(screen.getByLabelText('Select Channels')).toBeChecked()
    expect(screen.getByLabelText('Select Goroutines')).toBeChecked()
  })

  it('shows an empty result without attempting to save', () => {
    // Given extraction found no candidates
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={[]} onClose={vi.fn()} />)

    // Then the empty state is shown and no save action exists
    expect(screen.getByText('No new knowledge found')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save as drafts' })).not.toBeInTheDocument()
  })

  it('closes when the dialog close control is used', async () => {
    // Given an open extraction dialog
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={[candidate('1', 'Channels')]}
        onClose={onClose}
      />,
    )

    // When dismissing it through the dialog close control
    await user.click(screen.getByRole('button', { name: 'Close' }))

    // Then the owner is notified exactly once and the batch is discarded
    expect(onClose).toHaveBeenCalledOnce()
    await waitFor(() => expect(discardExtraction).toHaveBeenCalledWith('batch-1'))
  })

  it('discards the batch when the Dismiss button is used', async () => {
    // Given an open extraction dialog
    const onClose = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={[candidate('1', 'Channels')]}
        onClose={onClose}
      />,
    )

    // When dismissing it via the explicit Dismiss button
    await user.click(screen.getByRole('button', { name: 'Dismiss' }))

    // Then the batch is discarded and the owner is notified — this is the
    // only way an undecided decision-zone row or an unselected batch-zone
    // row gets abandoned, since neither one calls the backend on its own
    await waitFor(() => expect(discardExtraction).toHaveBeenCalledWith('batch-1'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('disables saving when no batch candidate is checked', async () => {
    // Given one candidate that the user unchecks
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={[candidate('1', 'Channels')]}
        onClose={vi.fn()}
      />,
    )
    await user.click(screen.getByLabelText('Select Channels'))

    // Then saving is disabled
    expect(screen.getByRole('button', { name: 'Save as drafts' })).toBeDisabled()
  })

  it('restores a candidate to the selection when it is unchecked and checked again', async () => {
    // Given two candidates with the first temporarily unchecked
    const items = [candidate('1', 'One'), candidate('2', 'Two')]
    vi.mocked(applyReconciliationCreate).mockResolvedValue({ ...items[0], id: 'new-item' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)
    await user.click(screen.getByLabelText('Select One'))

    // When selecting it again and saving
    await user.click(screen.getByLabelText('Select One'))
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then both candidates are applied
    await waitFor(() => expect(applyReconciliationCreate).toHaveBeenCalledTimes(2))
  })

  it('saves only checked batch-zone candidates as drafts, via the reconciliation create binding', async () => {
    // Given two plain create candidates with the second unchecked
    const items = [candidate('1', 'Channels'), candidate('2', 'Goroutines')]
    vi.mocked(applyReconciliationCreate).mockResolvedValueOnce({ ...items[0], id: 'new-item-1' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)
    await user.click(screen.getByLabelText('Select Goroutines'))

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then only the checked candidate is applied, as a draft
    await waitFor(() =>
      expect(applyReconciliationCreate).toHaveBeenCalledWith('batch-1', '1', items[0], 'draft'),
    )
    expect(applyReconciliationCreate).toHaveBeenCalledOnce()
    expect(await screen.findByText('Saved')).toBeInTheDocument()
  })

  it('saves via "Save as knowledge" directly as approved', async () => {
    // Given one create candidate
    const items = [candidate('1', 'Channels')]
    vi.mocked(applyReconciliationCreate).mockResolvedValueOnce({ ...items[0], id: 'new-item-1' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // When saving via "Save as knowledge"
    await user.click(screen.getByRole('button', { name: 'Save as knowledge' }))

    // Then it is applied with status "approved"
    await waitFor(() =>
      expect(applyReconciliationCreate).toHaveBeenCalledWith('batch-1', '1', items[0], 'approved'),
    )
  })

  it('acknowledges a no_change batch candidate instead of creating an item', async () => {
    // Given a candidate the classifier says is already fully captured
    const items = [noChangeCandidate('1', 'Channels')]
    vi.mocked(acknowledgeReconciliationNoChange).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then it is acknowledged, never created
    await waitFor(() =>
      expect(acknowledgeReconciliationNoChange).toHaveBeenCalledWith('batch-1', '1', items[0]),
    )
    expect(applyReconciliationCreate).not.toHaveBeenCalled()
    expect(await screen.findByText('Acknowledged')).toBeInTheDocument()
  })

  it('stops the batch on the first failure, leaving it retryable and not discarding progress', async () => {
    // Given two candidates where the first save fails
    const items = [candidate('1', 'One'), candidate('2', 'Two')]
    vi.mocked(applyReconciliationCreate).mockRejectedValueOnce(new Error('database locked'))
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // When saving drafts
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then the error is shown and the batch stops — the second candidate is
    // never attempted
    expect(await screen.findByText('database locked')).toBeInTheDocument()
    expect(applyReconciliationCreate).toHaveBeenCalledOnce()
  })

  it('calls onKnowledgeChanged after a successful draft save, but not after "Save as knowledge"', async () => {
    // Given a create candidate
    const items = [candidate('1', 'Channels')]
    vi.mocked(applyReconciliationCreate).mockResolvedValue({ ...items[0], id: 'new-item-1' })
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When saving as drafts
    await user.click(screen.getByRole('button', { name: 'Save as drafts' }))

    // Then the badge-freshness callback fires — a new draft entered the queue
    await waitFor(() => expect(onKnowledgeChanged).toHaveBeenCalledTimes(1))
  })

  it('does not call onKnowledgeChanged for "Save as knowledge", which never creates a draft', async () => {
    // Given a create candidate saved directly as approved
    const items = [candidate('1', 'Channels')]
    vi.mocked(applyReconciliationCreate).mockResolvedValue({ ...items[0], id: 'new-item-1' })
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When saving via "Save as knowledge"
    await user.click(screen.getByRole('button', { name: 'Save as knowledge' }))

    await waitFor(() => expect(applyReconciliationCreate).toHaveBeenCalled())
    expect(onKnowledgeChanged).not.toHaveBeenCalled()
  })

  it('separates a candidate that updates an existing item into its own decision row', () => {
    // Given a candidate classified as an update against an existing item
    const items = [updateCandidate('1', 'Eventual consistency')]

    // When opening the dialog
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // Then it appears under "Needs your decision", with no batch checkbox,
    // showing the target it compares with and the proposed new definition
    expect(screen.getByText('Needs your decision')).toBeInTheDocument()
    expect(screen.queryByLabelText('Select Eventual consistency')).not.toBeInTheDocument()
    expect(screen.getByText(/Existing Eventual consistency/)).toBeInTheDocument()
    expect(screen.getByText(/A richer definition\./)).toBeInTheDocument()
  })

  it('applies an update decision immediately, independent of the batch buttons', async () => {
    // Given one update candidate and one plain create candidate
    const items = [updateCandidate('1', 'Eventual consistency'), candidate('2', 'Idempotency key')]
    vi.mocked(applyReconciliationUpdate).mockResolvedValueOnce({ ...items[0], id: 'existing-1' })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // When applying the update
    await user.click(screen.getByRole('button', { name: 'Apply update' }))

    // Then only that candidate's update binding is called, immediately —
    // no batch button was involved
    await waitFor(() =>
      expect(applyReconciliationUpdate).toHaveBeenCalledWith('batch-1', '1', items[0]),
    )
    expect(await screen.findByText('Applied')).toBeInTheDocument()
    expect(applyReconciliationCreate).not.toHaveBeenCalled()
  })

  it('saves a decision-zone candidate for review instead of applying it immediately', async () => {
    // Given an update candidate
    const items = [updateCandidate('1', 'Eventual consistency')]
    vi.mocked(saveReconciliationForReview).mockResolvedValueOnce(undefined)
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When saving it for review instead of applying it
    await user.click(screen.getByRole('button', { name: 'Save for review' }))

    // Then it is saved for later, never applied immediately, and the draft
    // badge callback never fires — nothing was actually created yet
    await waitFor(() =>
      expect(saveReconciliationForReview).toHaveBeenCalledWith('batch-1', '1', items[0]),
    )
    expect(applyReconciliationUpdate).not.toHaveBeenCalled()
    expect(await screen.findByText('Saved for review')).toBeInTheDocument()
    expect(onKnowledgeChanged).not.toHaveBeenCalled()
  })

  it('does not call onKnowledgeChanged after applying an update — the target keeps its own status', async () => {
    // Given an update candidate
    const items = [updateCandidate('1', 'Eventual consistency')]
    vi.mocked(applyReconciliationUpdate).mockResolvedValueOnce({ ...items[0], id: 'existing-1' })
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When applying the update
    await user.click(screen.getByRole('button', { name: 'Apply update' }))

    await waitFor(() => expect(applyReconciliationUpdate).toHaveBeenCalled())
    expect(onKnowledgeChanged).not.toHaveBeenCalled()
  })

  it('shows a retry button and keeps the row usable after a failed update', async () => {
    // Given an update whose apply call fails once
    const items = [updateCandidate('1', 'Eventual consistency')]
    vi.mocked(applyReconciliationUpdate).mockRejectedValueOnce(
      new Error('target changed since comparison'),
    )
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // When applying it and it fails
    await user.click(screen.getByRole('button', { name: 'Apply update' }))
    expect(await screen.findByText('target changed since comparison')).toBeInTheDocument()

    // Then the row offers a retry
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled()
  })

  it('creates and relates a candidate through its own decision row', async () => {
    // Given a candidate classified as related to an existing item
    const items = [relateCandidate('1', 'CAP theorem')]
    vi.mocked(applyReconciliationRelate).mockResolvedValueOnce({ ...items[0], id: 'new-item-1' })
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When creating and relating it
    await user.click(screen.getByRole('button', { name: 'Create & relate' }))

    // Then the relate binding is called and the draft badge callback fires
    // — a relate always creates a new draft item
    await waitFor(() =>
      expect(applyReconciliationRelate).toHaveBeenCalledWith('batch-1', '1', items[0]),
    )
    await waitFor(() => expect(onKnowledgeChanged).toHaveBeenCalledTimes(1))
  })

  it('offers all three explicit outcomes for a conflict, and none apply until chosen', () => {
    // Given a candidate whose content contradicts an existing item
    const items = [conflictCandidate('1', 'Circuit breaker')]

    // When opening the dialog
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // Then all three resolutions are offered
    expect(screen.getByRole('button', { name: 'Keep existing' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Update existing' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create separately' })).toBeInTheDocument()
  })

  it('resolves a conflict by keeping the existing item, without calling onKnowledgeChanged', async () => {
    // Given a conflict candidate
    const items = [conflictCandidate('1', 'Circuit breaker')]
    vi.mocked(resolveReconciliationConflict).mockResolvedValueOnce({} as KnowledgeItem)
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When choosing to keep the existing item
    await user.click(screen.getByRole('button', { name: 'Keep existing' }))

    // Then that exact resolution is sent, and no draft was added
    await waitFor(() =>
      expect(resolveReconciliationConflict).toHaveBeenCalledWith(
        'batch-1',
        '1',
        items[0],
        CONFLICT_KEEP_EXISTING,
      ),
    )
    expect(onKnowledgeChanged).not.toHaveBeenCalled()
  })

  it('resolves a conflict by updating the existing item', async () => {
    // Given a conflict candidate
    const items = [conflictCandidate('1', 'Circuit breaker')]
    vi.mocked(resolveReconciliationConflict).mockResolvedValueOnce({
      ...items[0],
      id: 'existing-3',
    })
    const user = userEvent.setup()
    render(<KnowledgeExtractionDialog open batchId="batch-1" items={items} onClose={vi.fn()} />)

    // When choosing to update the existing item
    await user.click(screen.getByRole('button', { name: 'Update existing' }))

    // Then that exact resolution is sent
    await waitFor(() =>
      expect(resolveReconciliationConflict).toHaveBeenCalledWith(
        'batch-1',
        '1',
        items[0],
        CONFLICT_UPDATE_EXISTING,
      ),
    )
  })

  it('resolves a conflict by creating separately, and calls onKnowledgeChanged since that always drafts', async () => {
    // Given a conflict candidate
    const items = [conflictCandidate('1', 'Circuit breaker')]
    vi.mocked(resolveReconciliationConflict).mockResolvedValueOnce({
      ...items[0],
      id: 'new-item-1',
    })
    const onKnowledgeChanged = vi.fn()
    const user = userEvent.setup()
    render(
      <KnowledgeExtractionDialog
        open
        batchId="batch-1"
        items={items}
        onClose={vi.fn()}
        onKnowledgeChanged={onKnowledgeChanged}
      />,
    )

    // When choosing to create separately
    await user.click(screen.getByRole('button', { name: 'Create separately' }))

    // Then that exact resolution is sent and the draft badge callback fires
    await waitFor(() =>
      expect(resolveReconciliationConflict).toHaveBeenCalledWith(
        'batch-1',
        '1',
        items[0],
        CONFLICT_CREATE_SEPARATELY,
      ),
    )
    await waitFor(() => expect(onKnowledgeChanged).toHaveBeenCalledTimes(1))
  })

  it('resets both zones for a new batch even if the component stays mounted', async () => {
    // Given a dialog open on one batch, with its single batch-zone candidate
    // manually deselected
    const firstBatch = [candidate('1', 'Channels')]
    const user = userEvent.setup()
    const { rerender } = render(
      <KnowledgeExtractionDialog open batchId="batch-1" items={firstBatch} onClose={vi.fn()} />,
    )
    await user.click(screen.getByLabelText('Select Channels'))
    expect(screen.getByLabelText('Select Channels')).not.toBeChecked()

    // When a new batch arrives with a different batchId, carrying an update
    // candidate and a plain create candidate, without the component ever
    // unmounting
    const secondBatch = [updateCandidate('2', 'Eventual consistency'), candidate('3', 'Goroutines')]
    rerender(
      <KnowledgeExtractionDialog open batchId="batch-2" items={secondBatch} onClose={vi.fn()} />,
    )

    // Then the new batch renders on its own merits — the update candidate in
    // its own decision row, the plain candidate selected as usual
    expect(screen.getByText('Needs your decision')).toBeInTheDocument()
    expect(screen.getByLabelText('Select Goroutines')).toBeChecked()
  })
})
