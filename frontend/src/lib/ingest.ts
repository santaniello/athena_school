import { ImportNotes, PickNotesFolder } from '../../wailsjs/go/desktop/App'
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
}

// pickNotesFolder opens the OS folder picker and returns the chosen path,
// or "" if the user cancelled.
export async function pickNotesFolder(): Promise<string> {
  return PickNotesFolder()
}

// importNotes starts importing path. It resolves once the import has
// finished (successfully or not) — progress and the final summary arrive
// separately via onIngestProgress/onIngestDone/onIngestError, so callers
// should subscribe to those before calling this.
export async function importNotes(path: string): Promise<void> {
  await ImportNotes(path)
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
