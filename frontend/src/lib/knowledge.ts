import {
  ExtractKnowledge,
  GetKnowledgeExtractionSettings,
  SaveExtractedKnowledge,
  UpdateKnowledgeExtractionSettings,
} from '../../wailsjs/go/desktop/App'

export interface KnowledgeItem {
  id: string
  topic: string
  concept: string
  definition: string
  properties: string[]
  tradeOffs: string[]
  relatedConcepts: string[]
  source: string
  status: string
  createdAt: string
  updatedAt: string
}

export interface ExtractionResult {
  items: KnowledgeItem[]
  truncated: boolean
}

export interface KnowledgeExtractionSettings {
  maxKnowledgeExtractionItems: number
}

export type KnowledgeSaveResult = Awaited<ReturnType<typeof SaveExtractedKnowledge>>

export async function extractKnowledge(
  sessionId: string,
  confirmedTruncation: boolean,
): Promise<ExtractionResult> {
  return ExtractKnowledge(sessionId, confirmedTruncation)
}

export async function saveExtractedKnowledge(items: KnowledgeItem[]): Promise<KnowledgeSaveResult> {
  return SaveExtractedKnowledge(items)
}

export async function getKnowledgeExtractionSettings(): Promise<KnowledgeExtractionSettings> {
  return GetKnowledgeExtractionSettings()
}

export async function updateKnowledgeExtractionSettings(maxItems: number): Promise<void> {
  await UpdateKnowledgeExtractionSettings(maxItems)
}
