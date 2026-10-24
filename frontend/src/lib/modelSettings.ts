import {
  GetModelSettings,
  ListSelectableModels,
  SaveModelSettings,
} from '../../wailsjs/go/desktop/App'

// Projected from the Wails-generated bindings instead of hand-duplicated, so
// a renamed field on the Go DTOs fails these files to compile instead of
// drifting behind a cast. An empty choice means "use the default". See
// specs/phases/phase-02-knowledge-engine/31-llm-model-param.md.
export type ModelChoices = Awaited<ReturnType<typeof GetModelSettings>>['current']
export type SelectableModels = Awaited<ReturnType<typeof ListSelectableModels>>

// The user's choices next to the defaults they fall back to.
export interface ModelSettings {
  current: ModelChoices
  defaults: ModelChoices
}

export type ChatModelKey = Exclude<keyof ModelChoices, 'embedding'>

export interface ModelFeature {
  key: ChatModelKey
  label: string
}

// The four chat features, in the order the Settings screen shows them. The
// embedding model is separate: it sits behind an "Advanced" warning.
export const MODEL_FEATURES: ModelFeature[] = [
  { key: 'study', label: 'Study chat' },
  { key: 'conversationSummary', label: 'Conversation summary' },
  { key: 'queryEnrichment', label: 'Query enrichment' },
  { key: 'noteFormatting', label: 'Note formatting (import)' },
]

export async function getModelSettings(): Promise<ModelSettings> {
  const { current, defaults } = await GetModelSettings()
  return { current, defaults }
}

export function saveModelSettings(
  choices: ModelChoices,
): Promise<{ embeddingModelChanged: boolean }> {
  return SaveModelSettings(choices)
}

export function listSelectableModels(): Promise<SelectableModels> {
  return ListSelectableModels()
}

const FALLBACK_MESSAGE = 'An error occurred. Please try again.'

// Wails rejects with the Go error's text, as a string or wrapped in an Error.
// These match the sentinels in internal/application/settings/errors.go; the
// text after them (the offending ID, the transport error) is not shown.
export function modelSettingsErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : ''
  if (raw.includes('model is not in the catalog')) {
    return 'That model is not available in the OpenRouter catalog.'
  }
  if (raw.includes('model catalog is unavailable')) {
    return 'Could not load the OpenRouter model catalog. Check your connection and try again.'
  }
  return FALLBACK_MESSAGE
}
