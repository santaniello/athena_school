import {
  GetSessionSourceDocument,
  ListSessionSources,
  RemoveSessionSource,
} from '../../wailsjs/go/desktop/App'

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

// DocumentSegment is one ordered slice of a SourceDocument's text: either a
// chunk's own passage (chunkId set) or the gap between two chunks (chunkId
// empty).
export interface DocumentSegment {
  text: string
  chunkId: string
}

// SourceDocument is a session's imported document, as read by the source
// viewer: its title/path plus its full stored text, already cut into
// ordered segments at each chunk's own boundaries.
export interface SourceDocument {
  itemId: string
  title: string
  path: string
  segments: DocumentSegment[]
}

// getSessionSourceDocument returns itemId's full document within sessionId,
// for the source viewer. Rejects if itemId does not exist, belongs to
// another session, or has no document text stored (a document imported
// before this feature, or whose text was otherwise lost).
export async function getSessionSourceDocument(
  sessionId: string,
  itemId: string,
): Promise<SourceDocument> {
  return GetSessionSourceDocument(sessionId, itemId)
}
