import {
  AcknowledgeReconciliationNoChange,
  ApplyReconciliationCreate,
  ApplyReconciliationRelate,
  ApplyReconciliationUpdate,
  ApproveKnowledgeItem,
  CountDraftKnowledgeItems,
  CountUnindexedKnowledgeItems,
  DeleteKnowledgeItem,
  DeprecateKnowledgeItem,
  DiscardExtraction,
  ExtractKnowledge,
  GetKnowledgeExtractionSettings,
  ListKnowledgeItemEvidence,
  ListKnowledgeItems,
  ListKnowledgeTopics,
  ReindexKnowledgeItems,
  ResolveReconciliationConflict,
  SaveAndApproveExtractedKnowledge,
  SaveExtractedKnowledge,
  SaveReconciliationForReview,
  UpdateKnowledgeExtractionSettings,
  UpdateKnowledgeItem,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

// The five reconciliation actions the classifier can propose, and the
// three explicit outcomes a conflict can resolve to — mirror
// domainknowledge.Reconcile*/applicationknowledge.Conflict* respectively.
export const RECONCILE_CREATE = 'create'
export const RECONCILE_UPDATE = 'update'
export const RECONCILE_RELATE = 'relate'
export const RECONCILE_CONFLICT = 'conflict'
export const RECONCILE_NO_CHANGE = 'no_change'

export const CONFLICT_KEEP_EXISTING = 'keep_existing'
export const CONFLICT_UPDATE_EXISTING = 'update_existing'
export const CONFLICT_CREATE_SEPARATELY = 'create_separately'

// MatchExact/MatchSemantic mirror domainknowledge.MatchExact/MatchSemantic.
export const MATCH_EXACT = 'exact'
export const MATCH_SEMANTIC = 'semantic'

export interface DuplicateMatch {
  itemId: string
  concept: string
  status: string
  matchType: string
  score: number
}

// ItemChanges mirrors domainknowledge.ItemChanges — optional replacements
// for an existing item's content fields. An absent field means "leave this
// field unchanged" when applying an update.
export interface ItemChanges {
  definition?: string
  properties: string[]
  tradeOffs: string[]
  relatedConcepts: string[]
}

// ReconciliationSuggestion is the classifier's suggested action for one
// extraction candidate — see RECONCILE_* for the possible action values.
// targetItemId, when set, is always one of the candidate's own duplicates
// entries; look it up there for the target's concept/status rather than
// duplicating those fields here.
export interface ReconciliationSuggestion {
  action: string
  targetItemId: string
  reason: string
  changes: ItemChanges
}

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
  // duplicates/semanticCheckUnavailable/reconciliation/reconciliationFailed
  // are only ever populated by extractKnowledge — every other producer of a
  // KnowledgeItem (list, approve, deprecate, update) omits them, since
  // duplicate detection and reconciliation classification both run at
  // extraction time only. Optional (rather than required null/false) so
  // every other screen's fixtures don't need to know about these fields.
  duplicates?: DuplicateMatch[] | null
  semanticCheckUnavailable?: boolean
  reconciliation?: ReconciliationSuggestion | null
  reconciliationFailed?: boolean
}

export interface ExtractionResult {
  batchId: string
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

// batchId identifies the backend extraction receipt returned by
// extractKnowledge — each item's id is used only as an opaque lookup key
// into that receipt; the backend, not this client, is authoritative for
// the evidence behind every saved item.
export async function saveExtractedKnowledge(
  batchId: string,
  items: KnowledgeItem[],
): Promise<KnowledgeSaveResult> {
  return SaveExtractedKnowledge(batchId, items)
}

// saveAndApproveExtractedKnowledge persists items directly as approved,
// skipping the draft review stage — the "Save as knowledge" option from
// specs/Athena.md §12, alongside saveExtractedKnowledge ("Save as drafts")
// and discarding the candidates entirely ("Dismiss").
export async function saveAndApproveExtractedKnowledge(
  batchId: string,
  items: KnowledgeItem[],
): Promise<KnowledgeSaveResult> {
  return SaveAndApproveExtractedKnowledge(batchId, items)
}

// discardExtraction drops every unsaved candidate in batchId — call this
// from every true Dismiss/dialog-close path, but never after a partial save
// error, so unsaved or failed candidates stay retryable.
export async function discardExtraction(batchId: string): Promise<void> {
  await DiscardExtraction(batchId)
}

// applyReconciliationCreate persists candidate as a brand-new Knowledge
// Item at status ('draft' | 'approved'), ignoring its client id — the
// backend regenerates it server-side. batchId/candidateId identify the
// backend's own classification receipt; the frontend is never trusted for
// provenance.
export async function applyReconciliationCreate(
  batchId: string,
  candidateId: string,
  candidate: KnowledgeItem,
  status: string,
): Promise<KnowledgeItem> {
  return ApplyReconciliationCreate(batchId, candidateId, candidate, status)
}

// applyReconciliationUpdate applies the classified changes to the
// candidate's target, preserving its identity and lifecycle.
export async function applyReconciliationUpdate(
  batchId: string,
  candidateId: string,
  candidate: KnowledgeItem,
): Promise<KnowledgeItem> {
  return ApplyReconciliationUpdate(batchId, candidateId, candidate)
}

// applyReconciliationRelate creates candidate as a new draft Knowledge Item
// and links it to the classified target via a `related` relation.
export async function applyReconciliationRelate(
  batchId: string,
  candidateId: string,
  candidate: KnowledgeItem,
): Promise<KnowledgeItem> {
  return ApplyReconciliationRelate(batchId, candidateId, candidate)
}

// resolveReconciliationConflict applies one of the three explicit conflict
// outcomes — see CONFLICT_KEEP_EXISTING/CONFLICT_UPDATE_EXISTING/
// CONFLICT_CREATE_SEPARATELY. The resolved item is empty for
// CONFLICT_KEEP_EXISTING, which never creates or changes anything.
export async function resolveReconciliationConflict(
  batchId: string,
  candidateId: string,
  candidate: KnowledgeItem,
  resolution: string,
): Promise<KnowledgeItem> {
  return ResolveReconciliationConflict(batchId, candidateId, candidate, resolution)
}

// acknowledgeReconciliationNoChange marks the candidate's classified
// no_change proposal resolved without creating or changing any item.
export async function acknowledgeReconciliationNoChange(
  batchId: string,
  candidateId: string,
  candidate: KnowledgeItem,
): Promise<void> {
  await AcknowledgeReconciliationNoChange(batchId, candidateId, candidate)
}

// saveReconciliationForReview persists the candidate's classified proposal
// as pending, changing neither the candidate nor its target — Knowledge
// Review is where it is later decided.
export async function saveReconciliationForReview(
  batchId: string,
  candidateId: string,
  candidate: KnowledgeItem,
): Promise<void> {
  await SaveReconciliationForReview(batchId, candidateId, candidate)
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

// countDraftKnowledgeItems returns how many Items currently have draft
// status, for the sidebar/Review-tab badge.
export async function countDraftKnowledgeItems(): Promise<number> {
  return CountDraftKnowledgeItems()
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

export async function updateKnowledgeItem(
  id: string,
  fields: KnowledgeItemEdit,
): Promise<KnowledgeItem> {
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

export interface KnowledgeEvidence {
  originType: string
  sourceLabel: string
  excerpt: string
  createdAt: string
}

// listKnowledgeItemEvidence returns id's persisted Evidence snapshots, in
// deterministic order. Empty for a legacy or shadow Item that never went
// through evidence-bearing extraction (e.g. an imported-note shadow Item).
export async function listKnowledgeItemEvidence(id: string): Promise<KnowledgeEvidence[]> {
  return ListKnowledgeItemEvidence(id)
}

export interface ReindexProgress {
  itemsProcessed: number
  itemsTotal: number
  currentTopic: string
}

export interface ReindexFailure {
  itemId: string
  topic: string
  reason: string
}

export interface ReindexSummary {
  itemsProcessed: number
  itemsIndexed: number
  itemsFailed: number
  failures: ReindexFailure[]
}

// countUnindexedKnowledgeItems returns how many Knowledge Items currently
// lack a current chunk for search — the count the Explorer's backfill
// Alert shows on mount.
export async function countUnindexedKnowledgeItems(): Promise<number> {
  return CountUnindexedKnowledgeItems()
}

// reindexKnowledgeItems starts processing every currently-unindexed
// Knowledge Item ("Index now"). It resolves once the run has finished
// (successfully or not) — progress and the final summary arrive separately
// via onReindexProgress/onReindexDone/onReindexError, which reuse the same
// ingest:* events 2.3 already streams (the UI only ever has one such
// operation active at a time), so callers should subscribe to those before
// calling this.
export async function reindexKnowledgeItems(): Promise<void> {
  await ReindexKnowledgeItems()
}

export function onReindexProgress(handler: (progress: ReindexProgress) => void): () => void {
  return EventsOn('ingest:progress', (progress: ReindexProgress) => handler(progress))
}

export function onReindexDone(handler: (summary: ReindexSummary) => void): () => void {
  return EventsOn('ingest:done', (summary: ReindexSummary) => handler(summary))
}

export function onReindexError(handler: (message: string) => void): () => void {
  return EventsOn('ingest:error', (message: string) => handler(message))
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
  if (max <= 0) {
    return ''
  }
  if (max === 1) {
    return '…'
  }
  // Reserve one character for the ellipsis so a truncated result never
  // exceeds max characters total.
  const cut = text.slice(0, max - 1)
  const lastSpace = cut.lastIndexOf(' ')
  const trimmed = lastSpace > 0 ? cut.slice(0, lastSpace) : cut
  return `${trimmed.trimEnd()}…`
}
