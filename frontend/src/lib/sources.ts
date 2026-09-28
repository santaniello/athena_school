import { ListSessionSources, RemoveSessionSource } from '../../wailsjs/go/desktop/App'

// SessionSource is one document a study session has imported, as listed by
// the Sources panel — distinct from StudySource (lib/study.ts), which is a
// citation attached to a single message.
export interface SessionSource {
  itemId: string
  title: string
  path: string
  chunkCount: number
  ingestedAt: string
}

// listSessionSources returns sessionId's imported documents, oldest-imported
// first.
export async function listSessionSources(sessionId: string): Promise<SessionSource[]> {
  return ListSessionSources(sessionId)
}

// removeSessionSource hard-deletes itemId's document from sessionId: its
// chunks, its knowledge item and its ingested-file record. It never touches
// the file on disk, and rejects if itemId belongs to another session.
export async function removeSessionSource(sessionId: string, itemId: string): Promise<void> {
  await RemoveSessionSource(sessionId, itemId)
}
