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
    vi.mocked(SaveExtractedKnowledge).mockResolvedValueOnce(1)

    // When saving it
    const count = await saveExtractedKnowledge(items)

    // Then no fields are dropped
    expect(SaveExtractedKnowledge).toHaveBeenCalledWith(items)
    expect(count).toBe(1)
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

  it('exposes the partial count from a failed save for safe retry', async () => {
    // Given the desktop binding reports one saved item before failure
    vi.mocked(SaveExtractedKnowledge).mockRejectedValueOnce(
      new Error('knowledge save failed after 1 items: database locked'),
    )

    // When saving candidates
    const promise = saveExtractedKnowledge([])

    // Then the rejection carries a typed partialCount for the dialog
    await expect(promise).rejects.toMatchObject({ partialCount: 1 })
  })
})
