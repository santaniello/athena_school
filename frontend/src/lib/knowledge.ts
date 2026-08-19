import {
  ApproveKnowledgeItem,
  DeleteKnowledgeItem,
  DeprecateKnowledgeItem,
  ExtractKnowledge,
  GetKnowledgeExtractionSettings,
  ListKnowledgeItems,
  ListKnowledgeTopics,
  SaveExtractedKnowledge,
  UpdateKnowledgeExtractionSettings,
  UpdateKnowledgeItem,
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

// listKnowledgeItems returns every Item matching topic/status. An empty
// topic or status means no constraint on that field — pass '' to list
// across all topics or all statuses.
export async function listKnowledgeItems(topic: string, status: string): Promise<KnowledgeItem[]> {
  return ListKnowledgeItems(topic, status)
}

// listKnowledgeTopics returns every distinct topic across all Items,
// alphabetically — used to build the Explorer's sidebar topic tree.
export async function listKnowledgeTopics(): Promise<string[]> {
  return ListKnowledgeTopics()
}

export async function approveKnowledgeItem(id: string): Promise<KnowledgeItem> {
  return ApproveKnowledgeItem(id)
}

export async function deprecateKnowledgeItem(id: string): Promise<KnowledgeItem> {
  return DeprecateKnowledgeItem(id)
}

// The user-editable fields of a knowledge Item — never id, source, status,
// createdAt or updatedAt, which are server-owned and lifecycle-managed.
export interface KnowledgeItemEdit {
  topic: string
  concept: string
  definition: string
  properties: string[]
  tradeOffs: string[]
  relatedConcepts: string[]
}

export async function updateKnowledgeItem(id: string, fields: KnowledgeItemEdit): Promise<KnowledgeItem> {
  return UpdateKnowledgeItem(id, {
    id: '',
    topic: fields.topic,
    concept: fields.concept,
    definition: fields.definition,
    properties: fields.properties,
    tradeOffs: fields.tradeOffs,
    relatedConcepts: fields.relatedConcepts,
    source: '',
    status: '',
    createdAt: '',
    updatedAt: '',
  })
}

// deleteKnowledgeItem permanently removes id and every chunk it owns. This
// cannot be undone.
export async function deleteKnowledgeItem(id: string): Promise<void> {
  await DeleteKnowledgeItem(id)
}

// groupByTopic buckets items by their topic, preserving each topic's
// first-seen order and each bucket's original item order — the shape the
// Explorer's list column and topic tree both need.
export function groupByTopic(items: KnowledgeItem[]): Map<string, KnowledgeItem[]> {
  const groups = new Map<string, KnowledgeItem[]>()
  for (const item of items) {
    const bucket = groups.get(item.topic)
    if (bucket) {
      bucket.push(item)
    } else {
      groups.set(item.topic, [item])
    }
  }
  return groups
}

// definitionPreview truncates text to at most max characters, cutting on a
// word boundary and appending an ellipsis — used to render a short list
// preview of a Definition regardless of source (plain text for both
// Athena-extracted and imported-note items). Text at or under the budget
// is returned unchanged.
export function definitionPreview(text: string, max: number): string {
  if (text.length <= max) {
    return text
  }
  const cut = text.slice(0, max)
  const lastSpace = cut.lastIndexOf(' ')
  const trimmed = lastSpace > 0 ? cut.slice(0, lastSpace) : cut
  return `${trimmed.trimEnd()}…`
}
