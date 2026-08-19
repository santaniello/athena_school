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

export async function extractKnowledge(
  sessionId: string,
  confirmedTruncation: boolean,
): Promise<ExtractionResult> {
  return ExtractKnowledge(sessionId, confirmedTruncation)
}

export async function saveExtractedKnowledge(items: KnowledgeItem[]): Promise<number> {
  try {
    return await SaveExtractedKnowledge(items)
  } catch (caught) {
    if (caught instanceof Error) {
      const match = caught.message.match(/knowledge save failed after (\d+) items:/)
      if (match) Object.assign(caught, { partialCount: Number(match[1]) })
    }
    throw caught
  }
}

export async function getKnowledgeExtractionSettings(): Promise<KnowledgeExtractionSettings> {
  return GetKnowledgeExtractionSettings()
}

export async function updateKnowledgeExtractionSettings(maxItems: number): Promise<void> {
  await UpdateKnowledgeExtractionSettings(maxItems)
}
