import { GetKnowledgeIndexStatus, RetryKnowledgeIndex } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type IndexState = 'loading' | 'ready' | 'ready_with_warnings' | 'failed'

export interface ChunkLoadIssue {
  chunkId: string
  itemId: string
  source: string
  filePath: string
  reason: string
}

export interface IndexStatus {
  state: IndexState
  hasSnapshot: boolean
  issues: ChunkLoadIssue[]
  lastError: string
}

// getKnowledgeIndexStatus returns the vector index coordinator's current
// lifecycle snapshot.
export async function getKnowledgeIndexStatus(): Promise<IndexStatus> {
  return GetKnowledgeIndexStatus() as Promise<IndexStatus>
}

// retryKnowledgeIndex rebuilds a separate snapshot from SQLite. The
// previous snapshot (and its old issues, on failure) keeps serving search
// until this resolves — callers do not need to also apply the resolved
// value themselves, since "knowledge-index:status" fires with the same
// outcome (see onKnowledgeIndexStatus).
export async function retryKnowledgeIndex(): Promise<IndexStatus> {
  return RetryKnowledgeIndex() as Promise<IndexStatus>
}

// EventsOn returns its own unsubscribe function. Callers must invoke it on
// unmount to avoid leaking a listener (same discipline as the ingest:*/
// study:* events in lib/ingest.ts and lib/study.ts).
export function onKnowledgeIndexStatus(handler: (status: IndexStatus) => void): () => void {
  return EventsOn('knowledge-index:status', (status: IndexStatus) => handler(status))
}
