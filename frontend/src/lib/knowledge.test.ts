import { describe, expect, it, vi } from 'vitest'
import {
  GetKnowledgeExtractionSettings,
  UpdateKnowledgeExtractionSettings,
} from '../../wailsjs/go/desktop/App'
import { getKnowledgeExtractionSettings, updateKnowledgeExtractionSettings } from './knowledge'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetKnowledgeExtractionSettings: vi.fn(),
  UpdateKnowledgeExtractionSettings: vi.fn(),
}))

describe('knowledge bindings', () => {
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
})
