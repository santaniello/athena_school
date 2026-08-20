import { GetKnowledgeIndexStatus, RetryKnowledgeIndex } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type IndexState = 'loading' | 'ready' | 'ready_with_warnings' | 'failed'

// Projected from the Wails-generated binding's return type instead of
// hand-duplicated, so a renamed/removed field on the Go DTO fails this file
// to compile instead of silently drifting behind a cast.
type IndexStatusResponse = Pick<
  Awaited<ReturnType<typeof GetKnowledgeIndexStatus>>,
  'state' | 'hasSnapshot' | 'issues' | 'lastError'
>

export type ChunkLoadIssue = IndexStatusResponse['issues'][number]

export interface IndexStatus extends Omit<IndexStatusResponse, 'state'> {
  state: IndexState
}

// getKnowledgeIndexStatus returns the vector index coordinator's current
// lifecycle snapshot.
export async function getKnowledgeIndexStatus(): Promise<IndexStatus> {
  const result = await GetKnowledgeIndexStatus()
  return { ...result, state: result.state as IndexState }
}

// retryKnowledgeIndex rebuilds a separate snapshot from SQLite. The
// previous snapshot (and its old issues, on failure) keeps serving search
// until this resolves — callers do not need to also apply the resolved
// value themselves, since "knowledge-index:status" fires with the same
// outcome (see onKnowledgeIndexStatus).
export async function retryKnowledgeIndex(): Promise<IndexStatus> {
  const result = await RetryKnowledgeIndex()
  return { ...result, state: result.state as IndexState }
}

// EventsOn returns its own unsubscribe function. Callers must invoke it on
// unmount to avoid leaking a listener (same discipline as the ingest:*/
// study:* events in lib/ingest.ts and lib/study.ts).
export function onKnowledgeIndexStatus(handler: (status: IndexStatus) => void): () => void {
  return EventsOn('knowledge-index:status', (status: IndexStatus) => handler(status))
}
