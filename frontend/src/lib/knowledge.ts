import {
  GetKnowledgeExtractionSettings,
  UpdateKnowledgeExtractionSettings,
} from '../../wailsjs/go/desktop/App'

export interface KnowledgeExtractionSettings {
  maxKnowledgeExtractionItems: number
}

export async function getKnowledgeExtractionSettings(): Promise<KnowledgeExtractionSettings> {
  return GetKnowledgeExtractionSettings()
}

export async function updateKnowledgeExtractionSettings(maxItems: number): Promise<void> {
  await UpdateKnowledgeExtractionSettings(maxItems)
}
