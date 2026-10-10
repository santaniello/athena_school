import { DownloadExampleNote, ImportFile, PickNotesFile } from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export interface IngestProgress {
  filesProcessed: number
  filesTotal: number
  chunksCreated: number
  currentFile: string
}

export interface IngestFailure {
  path: string
  reason: string
}

export interface IngestSummary {
  filesScanned: number
  filesIngested: number
  filesSkipped: number
  filesFailed: number
  chunksCreated: number
  failures: IngestFailure[]
  // Files that persisted successfully (counted in filesIngested, never in
  // filesFailed) but whose in-memory vector index reconciliation failed. A
  // full Retry from the Knowledge section self-heals these from SQLite.
  indexWarnings: IngestFailure[]
}

// pickNotesFile opens the OS file picker restricted to .md/.txt files and
// returns the chosen path, or "" if the user cancelled.
export async function pickNotesFile(): Promise<string> {
  return PickNotesFile()
}

// downloadExampleNote opens the OS save dialog and writes a bundled
// example note demonstrating the heading/one-paragraph-per-sub-topic
// structure chunking works best with. Returns the chosen path, or "" if
// the user cancelled. See
// specs/phases/phase-02-knowledge-engine/19-example-note-template.md.
export async function downloadExampleNote(): Promise<string> {
  return DownloadExampleNote()
}

// importFile starts importing exactly one file at path into the study
// session sessionId, which owns the resulting knowledge. It resolves once
// the import has finished (successfully or not) — progress and the final
// summary arrive separately via onIngestProgress/onIngestDone/
// onIngestError, so callers should subscribe to those before calling this.
export async function importFile(sessionId: string, path: string): Promise<void> {
  await ImportFile(sessionId, path)
}

// EventsOn returns its own unsubscribe function. Callers must invoke it on
// unmount to avoid leaking a listener across dialog opens (same discipline
// as the study:* events in lib/study.ts).
export function onIngestProgress(handler: (progress: IngestProgress) => void): () => void {
  return EventsOn('ingest:progress', (progress: IngestProgress) => handler(progress))
}

export function onIngestDone(handler: (summary: IngestSummary) => void): () => void {
  return EventsOn('ingest:done', (summary: IngestSummary) => handler(summary))
}

export function onIngestError(handler: (message: string) => void): () => void {
  return EventsOn('ingest:error', (message: string) => handler(message))
}
