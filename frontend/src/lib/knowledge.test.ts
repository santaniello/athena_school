import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  AcknowledgePendingReconciliationNoChange,
  AcknowledgeReconciliationNoChange,
  ApplyPendingReconciliationCreate,
  ApplyPendingReconciliationRelate,
  ApplyPendingReconciliationUpdate,
  ApplyReconciliationCreate,
  ApplyReconciliationRelate,
  ApplyReconciliationUpdate,
  ApproveKnowledgeItem,
  CountDraftKnowledgeItems,
  CountPendingReconciliations,
  CountUnindexedKnowledgeItems,
  DeleteKnowledgeItem,
  DeprecateKnowledgeItem,
  DiscardExtraction,
  ExtractKnowledge,
  GetKnowledgeExtractionSettings,
  ListKnowledgeItemEvidence,
  ListKnowledgeItems,
  ListKnowledgeTopics,
  ListPendingReconciliations,
  RejectPendingReconciliationProposal,
  ReindexKnowledgeItems,
  ResolvePendingReconciliationConflict,
  ResolveReconciliationConflict,
  SaveAndApproveExtractedKnowledge,
  SaveExtractedKnowledge,
  SaveReconciliationForReview,
  UpdateKnowledgeExtractionSettings,
  UpdateKnowledgeItem,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  acknowledgePendingReconciliationNoChange,
  acknowledgeReconciliationNoChange,
  applyPendingReconciliationCreate,
  applyPendingReconciliationRelate,
  applyPendingReconciliationUpdate,
  applyReconciliationCreate,
  applyReconciliationRelate,
  applyReconciliationUpdate,
  approveKnowledgeItem,
  CONFLICT_CREATE_SEPARATELY,
  CONFLICT_KEEP_EXISTING,
  CONFLICT_UPDATE_EXISTING,
  countDraftKnowledgeItems,
  countPendingReconciliations,
  countUnindexedKnowledgeItems,
  definitionPreview,
  deleteKnowledgeItem,
  deprecateKnowledgeItem,
  discardExtraction,
  extractKnowledge,
  getKnowledgeExtractionSettings,
  groupByTopic,
  listKnowledgeItemEvidence,
  listKnowledgeItems,
  listKnowledgeTopics,
  listPendingReconciliations,
  MATCH_EXACT,
  MATCH_SEMANTIC,
  onReindexDone,
  onReindexError,
  onReindexProgress,
  RECONCILE_CREATE,
  RECONCILE_NO_CHANGE,
  reindexKnowledgeItems,
  rejectPendingReconciliationProposal,
  resolvePendingReconciliationConflict,
  resolveReconciliationConflict,
  saveAndApproveExtractedKnowledge,
  saveExtractedKnowledge,
  saveReconciliationForReview,
  updateKnowledgeExtractionSettings,
  updateKnowledgeItem,
  type KnowledgeItem,
} from './knowledge'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  ExtractKnowledge: vi.fn(),
  SaveExtractedKnowledge: vi.fn(),
  SaveAndApproveExtractedKnowledge: vi.fn(),
  DiscardExtraction: vi.fn(),
  GetKnowledgeExtractionSettings: vi.fn(),
  UpdateKnowledgeExtractionSettings: vi.fn(),
  ListKnowledgeItems: vi.fn(),
  ListKnowledgeTopics: vi.fn(),
  ListKnowledgeItemEvidence: vi.fn(),
  CountDraftKnowledgeItems: vi.fn(),
  CountUnindexedKnowledgeItems: vi.fn(),
  ReindexKnowledgeItems: vi.fn(),
  ApproveKnowledgeItem: vi.fn(),
  DeprecateKnowledgeItem: vi.fn(),
  UpdateKnowledgeItem: vi.fn(),
  DeleteKnowledgeItem: vi.fn(),
  ApplyReconciliationCreate: vi.fn(),
  ApplyReconciliationUpdate: vi.fn(),
  ApplyReconciliationRelate: vi.fn(),
  ResolveReconciliationConflict: vi.fn(),
  AcknowledgeReconciliationNoChange: vi.fn(),
  SaveReconciliationForReview: vi.fn(),
  ListPendingReconciliations: vi.fn(),
  CountPendingReconciliations: vi.fn(),
  ApplyPendingReconciliationCreate: vi.fn(),
  ApplyPendingReconciliationUpdate: vi.fn(),
  ApplyPendingReconciliationRelate: vi.fn(),
  ResolvePendingReconciliationConflict: vi.fn(),
  AcknowledgePendingReconciliationNoChange: vi.fn(),
  RejectPendingReconciliationProposal: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
}))

describe('knowledge bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('extracts knowledge with the truncation confirmation flag', async () => {
    // Given a backend extraction result
    vi.mocked(ExtractKnowledge).mockResolvedValueOnce({
      batchId: 'batch-1',
      items: [],
      truncated: true,
    } as never)

    // When extracting from a session
    const result = await extractKnowledge('session-1', false)

    // Then the full result and call arguments are preserved
    expect(ExtractKnowledge).toHaveBeenCalledWith('session-1', false)
    expect(result).toEqual({ batchId: 'batch-1', items: [], truncated: true })
  })

  it('saves the selected full candidates against their batch', async () => {
    // Given a complete candidate
    const items = [
      {
        id: 'candidate-1',
        topic: 'Go',
        concept: 'Channels',
        definition: 'Typed conduits.',
        properties: ['typed'],
        tradeOffs: ['coordination'],
        relatedConcepts: ['goroutines'],
        source: 'athena',
        status: 'draft',
        createdAt: '2026-08-18T10:00:00Z',
        updatedAt: '2026-08-18T10:00:00Z',
      },
    ]
    vi.mocked(SaveExtractedKnowledge).mockResolvedValueOnce({ savedIndices: [0], error: '' })

    // When saving it
    const result = await saveExtractedKnowledge('batch-1', items)

    // Then the batch ID and every field are forwarded
    expect(SaveExtractedKnowledge).toHaveBeenCalledWith('batch-1', items)
    expect(result).toEqual({ savedIndices: [0], error: '' })
  })

  it('saves and approves the selected full candidates against their batch', async () => {
    // Given a complete candidate
    const items = [
      {
        id: 'candidate-1',
        topic: 'Go',
        concept: 'Channels',
        definition: 'Typed conduits.',
        properties: ['typed'],
        tradeOffs: ['coordination'],
        relatedConcepts: ['goroutines'],
        source: 'athena',
        status: 'draft',
        createdAt: '2026-08-18T10:00:00Z',
        updatedAt: '2026-08-18T10:00:00Z',
      },
    ]
    vi.mocked(SaveAndApproveExtractedKnowledge).mockResolvedValueOnce({
      savedIndices: [0],
      error: '',
    })

    // When saving and approving it
    const result = await saveAndApproveExtractedKnowledge('batch-1', items)

    // Then the batch ID and every field are forwarded
    expect(SaveAndApproveExtractedKnowledge).toHaveBeenCalledWith('batch-1', items)
    expect(result).toEqual({ savedIndices: [0], error: '' })
  })

  it('discards a batch', async () => {
    // Given a backend that accepts the discard
    vi.mocked(DiscardExtraction).mockResolvedValueOnce()

    // When discarding a batch
    await discardExtraction('batch-1')

    // Then the batch ID was forwarded
    expect(DiscardExtraction).toHaveBeenCalledWith('batch-1')
  })

  it('reads and updates extraction settings', async () => {
    // Given a configured maximum
    vi.mocked(GetKnowledgeExtractionSettings).mockResolvedValueOnce({
      maxKnowledgeExtractionItems: 8,
    })
    vi.mocked(UpdateKnowledgeExtractionSettings).mockResolvedValueOnce()

    // When reading and updating it
    const settings = await getKnowledgeExtractionSettings()
    await updateKnowledgeExtractionSettings(12)

    // Then both bindings are used
    expect(settings.maxKnowledgeExtractionItems).toBe(8)
    expect(UpdateKnowledgeExtractionSettings).toHaveBeenCalledWith(12)
  })

  it('returns exact saved indices alongside a partial failure', async () => {
    // Given the desktop binding reports a non-prefix save before failure
    vi.mocked(SaveExtractedKnowledge).mockResolvedValueOnce({
      savedIndices: [1],
      error: 'knowledge save failed: database locked',
    })

    // When saving candidates
    const result = await saveExtractedKnowledge('batch-1', [])

    // Then the typed result is forwarded without parsing error text
    expect(result).toEqual({
      savedIndices: [1],
      error: 'knowledge save failed: database locked',
    })
  })

  it('preserves an ordinary binding rejection without inventing save metadata', async () => {
    // Given an unrelated backend failure
    const failure = new Error('database unavailable')
    vi.mocked(SaveExtractedKnowledge).mockRejectedValueOnce(failure)

    // When saving candidates
    const promise = saveExtractedKnowledge('batch-1', [])

    // Then the original error is preserved without partial-save metadata
    await expect(promise).rejects.toBe(failure)
    expect(failure).not.toHaveProperty('partialCount')
  })

  it('preserves a non-error rejection value', async () => {
    // Given an unexpected binding rejection
    vi.mocked(SaveExtractedKnowledge).mockRejectedValueOnce('unavailable')

    // When saving candidates
    const promise = saveExtractedKnowledge('batch-1', [])

    // Then the original rejection is propagated unchanged
    await expect(promise).rejects.toBe('unavailable')
  })
})

function testItem(overrides: Partial<KnowledgeItem> = {}): KnowledgeItem {
  return {
    id: 'item-1',
    topic: 'Go',
    concept: 'Channels',
    definition: 'Typed conduits.',
    properties: [],
    tradeOffs: [],
    relatedConcepts: [],
    source: 'athena',
    status: 'approved',
    createdAt: '2026-08-18T10:00:00Z',
    updatedAt: '2026-08-18T10:00:00Z',
    ...overrides,
  }
}

describe('listKnowledgeItems', () => {
  beforeEach(() => vi.clearAllMocks())

  it('forwards the topic and status filter and returns the items', async () => {
    // Given a backend list result
    const items = [testItem()]
    vi.mocked(ListKnowledgeItems).mockResolvedValueOnce(items as never)

    // When listing items
    const result = await listKnowledgeItems('Go', 'approved')

    // Then the filter is forwarded and the result returned as-is
    expect(ListKnowledgeItems).toHaveBeenCalledWith('Go', 'approved')
    expect(result).toEqual(items)
  })
})

describe('listKnowledgeItemEvidence', () => {
  beforeEach(() => vi.clearAllMocks())

  it('forwards the item id and returns its evidence snapshots', async () => {
    // Given a backend evidence list
    const evidence = [
      {
        originType: 'session_message',
        sourceLabel: 'Distributed systems',
        excerpt: 'CAP describes trade-offs.',
        createdAt: '2026-08-26T10:00:00Z',
      },
    ]
    vi.mocked(ListKnowledgeItemEvidence).mockResolvedValueOnce(evidence)

    // When listing an item's evidence
    const result = await listKnowledgeItemEvidence('item-1')

    // Then the id is forwarded and the snapshots are returned as-is
    expect(ListKnowledgeItemEvidence).toHaveBeenCalledWith('item-1')
    expect(result).toEqual(evidence)
  })
})

describe('listKnowledgeTopics', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns every topic', async () => {
    // Given a backend topic list
    vi.mocked(ListKnowledgeTopics).mockResolvedValueOnce(['Go', 'Kubernetes'])

    // When listing topics
    const topics = await listKnowledgeTopics()

    // Then they are returned as-is
    expect(topics).toEqual(['Go', 'Kubernetes'])
  })
})

describe('countDraftKnowledgeItems', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns the draft count', async () => {
    // Given a backend draft count
    vi.mocked(CountDraftKnowledgeItems).mockResolvedValueOnce(3)

    // When counting drafts
    const count = await countDraftKnowledgeItems()

    // Then it is returned as-is
    expect(count).toEqual(3)
  })
})

describe('approveKnowledgeItem', () => {
  beforeEach(() => vi.clearAllMocks())

  it('forwards the id and returns the updated item', async () => {
    // Given a backend approval result
    const item = testItem({ status: 'approved' })
    vi.mocked(ApproveKnowledgeItem).mockResolvedValueOnce(item as never)

    // When approving an item
    const result = await approveKnowledgeItem('item-1')

    // Then the id is forwarded and the updated item returned
    expect(ApproveKnowledgeItem).toHaveBeenCalledWith('item-1')
    expect(result).toEqual(item)
  })
})

describe('deprecateKnowledgeItem', () => {
  beforeEach(() => vi.clearAllMocks())

  it('forwards the id and returns the updated item', async () => {
    // Given a backend deprecation result
    const item = testItem({ status: 'deprecated' })
    vi.mocked(DeprecateKnowledgeItem).mockResolvedValueOnce(item as never)

    // When deprecating an item
    const result = await deprecateKnowledgeItem('item-1')

    // Then the id is forwarded and the updated item returned
    expect(DeprecateKnowledgeItem).toHaveBeenCalledWith('item-1')
    expect(result).toEqual(item)
  })
})

describe('updateKnowledgeItem', () => {
  beforeEach(() => vi.clearAllMocks())

  it('forwards only the editable fields, with placeholders for server-owned ones', async () => {
    // Given a backend update result
    const item = testItem({ concept: 'New concept' })
    vi.mocked(UpdateKnowledgeItem).mockResolvedValueOnce(item as never)

    // When updating an item's editable fields
    const result = await updateKnowledgeItem('item-1', {
      topic: 'Go',
      concept: 'New concept',
      definition: 'New definition.',
      properties: ['p1'],
      tradeOffs: ['t1'],
      relatedConcepts: ['r1'],
    })

    // Then the call carries exactly the editable fields, and the updated
    // item is returned
    expect(UpdateKnowledgeItem).toHaveBeenCalledWith('item-1', {
      id: '',
      topic: 'Go',
      concept: 'New concept',
      definition: 'New definition.',
      properties: ['p1'],
      tradeOffs: ['t1'],
      relatedConcepts: ['r1'],
      source: '',
      status: '',
      createdAt: '',
      updatedAt: '',
    })
    expect(result).toEqual(item)
  })
})

describe('deleteKnowledgeItem', () => {
  beforeEach(() => vi.clearAllMocks())

  it('forwards the id', async () => {
    // Given a delete call that succeeds
    vi.mocked(DeleteKnowledgeItem).mockResolvedValueOnce()

    // When deleting an item
    await deleteKnowledgeItem('item-1')

    // Then the id was forwarded
    expect(DeleteKnowledgeItem).toHaveBeenCalledWith('item-1')
  })
})

describe('reconciliation constants', () => {
  it('matches the domain and application layer values they mirror', () => {
    expect(RECONCILE_CREATE).toBe('create')
    expect(RECONCILE_NO_CHANGE).toBe('no_change')
    expect(CONFLICT_KEEP_EXISTING).toBe('keep_existing')
    expect(CONFLICT_UPDATE_EXISTING).toBe('update_existing')
    expect(CONFLICT_CREATE_SEPARATELY).toBe('create_separately')
    expect(MATCH_EXACT).toBe('exact')
    expect(MATCH_SEMANTIC).toBe('semantic')
  })
})

describe('immediate reconciliation bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('applies a create decision at the given status and returns the new item', async () => {
    // Given a backend create result
    const item = testItem({ id: 'new-item' })
    vi.mocked(ApplyReconciliationCreate).mockResolvedValueOnce(item as never)

    // When applying create
    const result = await applyReconciliationCreate('batch-1', 'candidate-1', testItem(), 'draft')

    // Then every argument is forwarded and the new item returned
    expect(ApplyReconciliationCreate).toHaveBeenCalledWith(
      'batch-1',
      'candidate-1',
      testItem(),
      'draft',
    )
    expect(result).toEqual(item)
  })

  it('applies an update decision and returns the updated item', async () => {
    // Given a backend update result
    const item = testItem({ concept: 'Updated' })
    vi.mocked(ApplyReconciliationUpdate).mockResolvedValueOnce(item as never)

    // When applying update
    const result = await applyReconciliationUpdate('batch-1', 'candidate-1', testItem())

    // Then every argument is forwarded and the updated item returned
    expect(ApplyReconciliationUpdate).toHaveBeenCalledWith('batch-1', 'candidate-1', testItem())
    expect(result).toEqual(item)
  })

  it('applies a relate decision and returns the new item', async () => {
    // Given a backend relate result
    const item = testItem({ id: 'new-item' })
    vi.mocked(ApplyReconciliationRelate).mockResolvedValueOnce(item as never)

    // When applying relate
    const result = await applyReconciliationRelate('batch-1', 'candidate-1', testItem())

    // Then every argument is forwarded and the new item returned
    expect(ApplyReconciliationRelate).toHaveBeenCalledWith('batch-1', 'candidate-1', testItem())
    expect(result).toEqual(item)
  })

  it('resolves a conflict with the given resolution and returns the result item', async () => {
    // Given a backend conflict resolution result
    const item = testItem({ concept: 'Resolved' })
    vi.mocked(ResolveReconciliationConflict).mockResolvedValueOnce(item as never)

    // When resolving the conflict
    const result = await resolveReconciliationConflict(
      'batch-1',
      'candidate-1',
      testItem(),
      CONFLICT_KEEP_EXISTING,
    )

    // Then every argument is forwarded and the result item returned
    expect(ResolveReconciliationConflict).toHaveBeenCalledWith(
      'batch-1',
      'candidate-1',
      testItem(),
      CONFLICT_KEEP_EXISTING,
    )
    expect(result).toEqual(item)
  })

  it('acknowledges a no_change decision', async () => {
    // Given a backend that accepts the acknowledgement
    vi.mocked(AcknowledgeReconciliationNoChange).mockResolvedValueOnce()

    // When acknowledging it
    await acknowledgeReconciliationNoChange('batch-1', 'candidate-1', testItem())

    // Then every argument was forwarded
    expect(AcknowledgeReconciliationNoChange).toHaveBeenCalledWith(
      'batch-1',
      'candidate-1',
      testItem(),
    )
  })

  it('saves a decision for later review', async () => {
    // Given a backend that accepts the save
    vi.mocked(SaveReconciliationForReview).mockResolvedValueOnce()

    // When saving it for review
    await saveReconciliationForReview('batch-1', 'candidate-1', testItem())

    // Then every argument was forwarded
    expect(SaveReconciliationForReview).toHaveBeenCalledWith('batch-1', 'candidate-1', testItem())
  })
})

describe('pending reconciliation bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists pending proposals as-is', async () => {
    // Given a backend list of pending proposals
    const proposals = [
      {
        id: 'proposal-1',
        action: 'update',
        candidate: testItem(),
        targetItemId: 'item-target',
        targetConcept: 'Channels',
        targetStatus: 'approved',
        reason: 'extends the existing definition',
        changes: {},
        stale: false,
        createdAt: '2026-08-28T09:00:00Z',
      },
    ]
    vi.mocked(ListPendingReconciliations).mockResolvedValueOnce(proposals as never)

    // When listing them
    const result = await listPendingReconciliations()

    // Then they are returned as-is
    expect(result).toEqual(proposals)
  })

  it('returns the pending count', async () => {
    // Given a backend pending count
    vi.mocked(CountPendingReconciliations).mockResolvedValueOnce(4)

    // When counting pending proposals
    const count = await countPendingReconciliations()

    // Then it is returned as-is
    expect(count).toBe(4)
  })

  it('applies a pending create decision at the given status', async () => {
    // Given a backend create result
    const item = testItem({ id: 'new-item' })
    vi.mocked(ApplyPendingReconciliationCreate).mockResolvedValueOnce(item as never)

    // When applying it
    const result = await applyPendingReconciliationCreate('proposal-1', 'approved')

    // Then the proposal id and status are forwarded and the item returned
    expect(ApplyPendingReconciliationCreate).toHaveBeenCalledWith('proposal-1', 'approved')
    expect(result).toEqual(item)
  })

  it('applies a pending update decision', async () => {
    // Given a backend update result
    const item = testItem({ concept: 'Updated' })
    vi.mocked(ApplyPendingReconciliationUpdate).mockResolvedValueOnce(item as never)

    // When applying it
    const result = await applyPendingReconciliationUpdate('proposal-1')

    // Then the proposal id is forwarded and the item returned
    expect(ApplyPendingReconciliationUpdate).toHaveBeenCalledWith('proposal-1')
    expect(result).toEqual(item)
  })

  it('applies a pending relate decision', async () => {
    // Given a backend relate result
    const item = testItem({ id: 'new-item' })
    vi.mocked(ApplyPendingReconciliationRelate).mockResolvedValueOnce(item as never)

    // When applying it
    const result = await applyPendingReconciliationRelate('proposal-1')

    // Then the proposal id is forwarded and the item returned
    expect(ApplyPendingReconciliationRelate).toHaveBeenCalledWith('proposal-1')
    expect(result).toEqual(item)
  })

  it('resolves a pending conflict with the given resolution', async () => {
    // Given a backend conflict resolution result
    const item = testItem({ concept: 'Resolved' })
    vi.mocked(ResolvePendingReconciliationConflict).mockResolvedValueOnce(item as never)

    // When resolving it
    const result = await resolvePendingReconciliationConflict(
      'proposal-1',
      CONFLICT_UPDATE_EXISTING,
    )

    // Then the proposal id and resolution are forwarded and the item returned
    expect(ResolvePendingReconciliationConflict).toHaveBeenCalledWith(
      'proposal-1',
      CONFLICT_UPDATE_EXISTING,
    )
    expect(result).toEqual(item)
  })

  it('acknowledges a pending no_change proposal', async () => {
    // Given a backend that accepts the acknowledgement
    vi.mocked(AcknowledgePendingReconciliationNoChange).mockResolvedValueOnce()

    // When acknowledging it
    await acknowledgePendingReconciliationNoChange('proposal-1')

    // Then the proposal id was forwarded
    expect(AcknowledgePendingReconciliationNoChange).toHaveBeenCalledWith('proposal-1')
  })

  it('rejects a pending proposal', async () => {
    // Given a backend that accepts the rejection
    vi.mocked(RejectPendingReconciliationProposal).mockResolvedValueOnce()

    // When rejecting it
    await rejectPendingReconciliationProposal('proposal-1')

    // Then the proposal id was forwarded
    expect(RejectPendingReconciliationProposal).toHaveBeenCalledWith('proposal-1')
  })
})

describe('groupByTopic', () => {
  it('buckets items by topic, preserving first-seen topic order and item order within each bucket', () => {
    // Given items across two topics, interleaved
    const goA = testItem({ id: 'go-a', topic: 'Go' })
    const k8sA = testItem({ id: 'k8s-a', topic: 'Kubernetes' })
    const goB = testItem({ id: 'go-b', topic: 'Go' })

    // When grouping by topic
    const groups = groupByTopic([goA, k8sA, goB])

    // Then topics appear in first-seen order, each with its items in
    // original order
    expect(Array.from(groups.keys())).toEqual(['Go', 'Kubernetes'])
    expect(groups.get('Go')).toEqual([goA, goB])
    expect(groups.get('Kubernetes')).toEqual([k8sA])
  })

  it('returns an empty map for an empty item list', () => {
    // Given no items
    // When grouping by topic
    const groups = groupByTopic([])

    // Then the result is an empty map
    expect(groups.size).toBe(0)
  })
})

describe('definitionPreview', () => {
  it('returns text unchanged when at or under the budget', () => {
    // Given text exactly at the budget
    const text = 'a'.repeat(10)

    // When previewing it at that same budget
    const result = definitionPreview(text, 10)

    // Then it is returned unchanged, with no ellipsis
    expect(result).toBe(text)
  })

  it('truncates on a word boundary and appends an ellipsis when over budget', () => {
    // Given text over budget where the cut point lands mid-word, one word
    // after the only space — "Hello wo" out of "Hello wonderful world"
    const text = 'Hello wonderful world'

    // When previewing it at a budget of 8
    const result = definitionPreview(text, 8)

    // Then it backs up to the space and drops the partial word entirely,
    // rather than keeping the mid-word fragment "wo"
    expect(result).toBe('Hello…')
  })

  it('drops every trailing space left by the cut, not just the last one', () => {
    // Given text with two consecutive spaces exactly at the cut point
    const text = 'AB  CD'

    // When previewing it at a budget of 4 (cutting right after both spaces)
    const result = definitionPreview(text, 4)

    // Then all trailing whitespace is trimmed before the ellipsis, not
    // just the one space the word-boundary search itself landed on
    expect(result).toBe('AB…')
  })

  it('keeps the hard cut when the only space in range sits at the very start', () => {
    // Given text that starts with a space, with no other space within the
    // cut region
    const text = ' abcdefgh'

    // When previewing it at a budget of 5
    const result = definitionPreview(text, 5)

    // Then a space at position 0 is not treated as a usable word boundary
    // (there is no word before it to keep) — the hard cut is kept instead,
    // one character short of the budget to leave room for the ellipsis
    expect(result).toBe(' abc…')
  })

  it('falls back to a hard cut when the budget has no whitespace to break on', () => {
    // Given a single long word with no spaces
    const text = 'a'.repeat(20)

    // When previewing it under budget
    const result = definitionPreview(text, 10)

    // Then it cuts one character short of the budget — nine characters,
    // not ten — so the appended ellipsis never pushes the result past max
    expect(result).toBe('aaaaaaaaa…')
  })

  it('returns just an ellipsis when max is 1', () => {
    // Given text well over budget and a budget too small to fit any
    // character alongside the ellipsis
    const result = definitionPreview('abcdef', 1)

    // Then only the ellipsis is returned
    expect(result).toBe('…')
  })

  it('returns an empty string when max is 0 or negative', () => {
    expect(definitionPreview('abcdef', 0)).toBe('')
    expect(definitionPreview('abcdef', -1)).toBe('')
  })

  it('never returns more than max characters total when truncating', () => {
    // Given text well over budget
    const text = 'a'.repeat(20)

    // When previewing it at a budget of 10
    const result = definitionPreview(text, 10)

    // Then the truncated result (including the ellipsis) fits the budget
    expect(result.length).toBeLessThanOrEqual(10)
  })
})

describe('countUnindexedKnowledgeItems', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns the unindexed count', async () => {
    // Given a backend unindexed count
    vi.mocked(CountUnindexedKnowledgeItems).mockResolvedValueOnce(5)

    // When counting unindexed items
    const count = await countUnindexedKnowledgeItems()

    // Then it is returned as-is
    expect(count).toEqual(5)
  })
})

describe('reindexKnowledgeItems', () => {
  beforeEach(() => vi.clearAllMocks())

  it('starts the reindex run', async () => {
    // Given a backend that accepts the run
    vi.mocked(ReindexKnowledgeItems).mockResolvedValueOnce(undefined)

    // When starting a reindex
    await reindexKnowledgeItems()

    // Then the binding was called
    expect(ReindexKnowledgeItems).toHaveBeenCalled()
  })
})

describe('onReindexProgress', () => {
  beforeEach(() => vi.clearAllMocks())

  it('subscribes to the ingest:progress event and forwards the payload', () => {
    // Given a handler and a mocked EventsOn
    const unsubscribe = vi.fn()
    vi.mocked(EventsOn).mockReturnValueOnce(unsubscribe)
    const handler = vi.fn()
    const progress = { itemsProcessed: 1, itemsTotal: 10, currentTopic: 'Go' }

    // When subscribing
    const result = onReindexProgress(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback(progress)

    // Then the handler received the progress payload, reusing the same
    // event 2.3's ingest already streams
    expect(EventsOn).toHaveBeenCalledWith('ingest:progress', expect.any(Function))
    expect(handler).toHaveBeenCalledWith(progress)
    expect(result).toBe(unsubscribe)
  })
})

describe('onReindexDone', () => {
  beforeEach(() => vi.clearAllMocks())

  it('subscribes to the ingest:done event and forwards the summary', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()
    const summary = { itemsProcessed: 10, itemsIndexed: 9, itemsFailed: 1, failures: [] }

    // When subscribing
    onReindexDone(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback(summary)

    // Then the handler received the summary
    expect(EventsOn).toHaveBeenCalledWith('ingest:done', expect.any(Function))
    expect(handler).toHaveBeenCalledWith(summary)
  })
})

describe('onReindexError', () => {
  beforeEach(() => vi.clearAllMocks())

  it('subscribes to the ingest:error event and forwards the message', () => {
    // Given a handler and a mocked EventsOn
    vi.mocked(EventsOn).mockReturnValueOnce(vi.fn())
    const handler = vi.fn()

    // When subscribing
    onReindexError(handler)
    const [, callback] = vi.mocked(EventsOn).mock.calls[0]
    callback('boom')

    // Then the handler received the message
    expect(EventsOn).toHaveBeenCalledWith('ingest:error', expect.any(Function))
    expect(handler).toHaveBeenCalledWith('boom')
  })
})
