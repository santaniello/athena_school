import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  ExtractKnowledge,
  GetKnowledgeExtractionSettings,
  SaveExtractedKnowledge,
  UpdateKnowledgeExtractionSettings,
} from '../../wailsjs/go/desktop/App'
import {
  extractKnowledge,
  getKnowledgeExtractionSettings,
  saveExtractedKnowledge,
  updateKnowledgeExtractionSettings,
} from './knowledge'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  ExtractKnowledge: vi.fn(),
  SaveExtractedKnowledge: vi.fn(),
  GetKnowledgeExtractionSettings: vi.fn(),
  UpdateKnowledgeExtractionSettings: vi.fn(),
}))

describe('knowledge bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('extracts knowledge with the truncation confirmation flag', async () => {
    // Given a backend extraction result
    vi.mocked(ExtractKnowledge).mockResolvedValueOnce({ items: [], truncated: true } as never)

    // When extracting from a session
    const result = await extractKnowledge('session-1', false)

    // Then the full result and call arguments are preserved
    expect(ExtractKnowledge).toHaveBeenCalledWith('session-1', false)
    expect(result).toEqual({ items: [], truncated: true })
  })

  it('saves the selected full candidates', async () => {
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
    const result = await saveExtractedKnowledge(items)

    // Then no fields are dropped
    expect(SaveExtractedKnowledge).toHaveBeenCalledWith(items)
    expect(result).toEqual({ savedIndices: [0], error: '' })
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
    const result = await saveExtractedKnowledge([])

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
    const promise = saveExtractedKnowledge([])

    // Then the original error is preserved without partial-save metadata
    await expect(promise).rejects.toBe(failure)
    expect(failure).not.toHaveProperty('partialCount')
  })

  it('preserves a non-error rejection value', async () => {
    // Given an unexpected binding rejection
    vi.mocked(SaveExtractedKnowledge).mockRejectedValueOnce('unavailable')

    // When saving candidates
    const promise = saveExtractedKnowledge([])

    // Then the original rejection is propagated unchanged
    await expect(promise).rejects.toBe('unavailable')
  })
})
