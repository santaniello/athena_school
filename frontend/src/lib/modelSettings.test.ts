import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  GetModelSettings,
  ListSelectableModels,
  SaveModelSettings,
} from '../../wailsjs/go/desktop/App'
import {
  getModelSettings,
  listSelectableModels,
  modelSettingsErrorMessage,
  saveModelSettings,
  MODEL_FEATURES,
} from './modelSettings'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetModelSettings: vi.fn(),
  SaveModelSettings: vi.fn(),
  ListSelectableModels: vi.fn(),
}))

const choices = {
  study: '',
  conversationSummary: '',
  queryEnrichment: '',
  noteFormatting: '',
  embedding: '',
}

describe('modelSettings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists the four chat features in the order the Settings screen shows them', () => {
    expect(MODEL_FEATURES.map((feature) => feature.key)).toEqual([
      'study',
      'conversationSummary',
      'queryEnrichment',
      'noteFormatting',
    ])
    expect(MODEL_FEATURES.map((feature) => feature.label)).toEqual([
      'Study chat',
      'Conversation summary',
      'Query enrichment',
      'Note formatting (import)',
    ])
  })

  it('reads the settings through the binding', async () => {
    const view = { current: choices, defaults: { ...choices, study: 'a/b' } }
    vi.mocked(GetModelSettings).mockResolvedValueOnce(view as never)

    await expect(getModelSettings()).resolves.toEqual(view)
  })

  it('saves the choices and reports whether the embedding model changed', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: true })

    const result = await saveModelSettings({ ...choices, embedding: 'x/y' })

    expect(SaveModelSettings).toHaveBeenCalledWith({ ...choices, embedding: 'x/y' })
    expect(result).toEqual({ embeddingModelChanged: true })
  })

  it('lists the selectable models', async () => {
    vi.mocked(ListSelectableModels).mockResolvedValueOnce({ chat: ['a/a'], embedding: ['e/e'] })

    await expect(listSelectableModels()).resolves.toEqual({ chat: ['a/a'], embedding: ['e/e'] })
  })

  it.each([
    [
      new Error('settings: model is not in the catalog: "made/up"'),
      'That model is not available in the OpenRouter catalog.',
    ],
    [
      'settings: model catalog is unavailable: offline',
      'Could not load the OpenRouter model catalog. Check your connection and try again.',
    ],
    [new Error('disk on fire'), 'An error occurred. Please try again.'],
    [undefined, 'An error occurred. Please try again.'],
  ])('maps %j to a user-facing message', (err, message) => {
    expect(modelSettingsErrorMessage(err)).toBe(message)
  })
})
