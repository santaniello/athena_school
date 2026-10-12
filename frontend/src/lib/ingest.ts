import {
  ConfirmImport,
  ConfirmLinkImport,
  DownloadExampleNote,
  ImportFile,
  PickNotesFile,
  PreviewImport,
  PreviewLinkImport,
} from '../../wailsjs/go/desktop/App'
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime'

export interface IngestProgress {
  filesProcessed: number
  filesTotal: number
  chunksCreated: number
  currentFile: string
  // Intra-file progress — only ConfirmImport (spec 2.20) ever sets these,
  // as each chunk of the single file it is confirming gets embedded.
  // Absent (or 0) on every tick ImportFile's own multi-file batch reports;
  // a reader falls back to filesProcessed/filesTotal in that case.
  chunksProcessed?: number
  chunksTotal?: number
}

// FormattingProgress is one "ingest:formatting-progress" tick — only
// previewImport (spec 2.20.1) emits these, as each heading-bounded batch of
// the file being previewed gets formatted.
export interface FormattingProgress {
  sectionsProcessed: number
  sectionsTotal: number
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

// ChunkPreview is one would-be chunk shown in the Review dialog before
// import is approved — the exact text confirmImport's chunker will also
// produce, since both run over the same approved content. See
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
export interface ChunkPreview {
  heading: string
  content: string
  charCount: number
  estimatedTokens: number
}

// FormattingStats are deterministic, verifiable structure counts comparing
// a note's original and formatted text — never a model's own narrated
// claim about what it changed.
export interface FormattingStats {
  headingsBefore: number
  headingsAfter: number
  paragraphsBefore: number
  paragraphsAfter: number
}

// FailedSection is one formatting batch that fell back to its own
// original content, naming which part of the note was affected instead
// of just the aggregate formattingFailed flag.
export interface FailedSection {
  heading: string
  reason: string
}

// StructureWarning flags one accepted batch (not a FailedSection — it
// passed fidelity validation) whose chunking outcome still looks
// unhealthy, e.g. a list of distinct items still grouped into one chunk.
export interface StructureWarning {
  heading: string
  message: string
}

// ImportPreview is previewImport's result. skipped is true when the file
// is unchanged since its last import — the Review dialog should not open
// for it at all.
export interface ImportPreview {
  displayPath: string
  // The canonical URL of a link preview (previewLinkImport); empty for a file.
  sourceUrl: string
  originalContent: string
  formattedContent: string
  formattingFailed: boolean
  failedSections: FailedSection[]
  structureWarnings: StructureWarning[]
  stats: FormattingStats
  chunks: ChunkPreview[]
  skipped: boolean
  skippedReason: string
}

// previewImport reformats and chunks the file at path for sessionId
// without persisting anything — no embeddings, no database write. Pass its
// formattedContent back into confirmImport once the user approves; never
// call confirmImport with anything else, or the chunks the user reviewed
// stop matching what actually gets persisted.
export async function previewImport(sessionId: string, path: string): Promise<ImportPreview> {
  return PreviewImport(sessionId, path)
}

// confirmImport persists exactly approvedContent — the formattedContent a
// prior previewImport call returned and the user approved — as path's
// chunks and shadow Item, owned by sessionId. Like importFile, it resolves
// once the import has finished; progress and the final summary arrive via
// the same onIngestProgress/onIngestDone/onIngestError events.
export async function confirmImport(
  sessionId: string,
  path: string,
  approvedContent: string,
): Promise<void> {
  await ConfirmImport(sessionId, path, approvedContent)
}

// previewLinkImport fetches the page at url and formats and chunks it for
// sessionId without persisting anything — the link counterpart of
// previewImport. Pass its sourceUrl, displayPath and formattedContent back
// into confirmLinkImport once the user approves.
export async function previewLinkImport(sessionId: string, url: string): Promise<ImportPreview> {
  return PreviewLinkImport(sessionId, url)
}

// confirmLinkImport persists exactly approvedContent — the formattedContent
// a prior previewLinkImport returned and the user approved — as url's
// source, owned by sessionId (the source is listed under its URL). Like confirmImport it resolves once the import
// has finished; progress and the final summary arrive via the same
// onIngestProgress/onIngestDone/onIngestError events.
export async function confirmLinkImport(
  sessionId: string,
  url: string,
  approvedContent: string,
): Promise<void> {
  await ConfirmLinkImport(sessionId, url, approvedContent)
}

// openOriginalLink opens a link source's page in the system browser.
export function openOriginalLink(url: string): void {
  BrowserOpenURL(url)
}

// linkErrorMessage turns a raw backend error from previewLinkImport into
// copy that names the actionable cause. Wails rejects with the error's
// message as a plain string, so both strings and Error instances are read.
export function linkErrorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : typeof error === 'string' ? error : ''
  if (raw.includes('host is not allowed')) {
    return "Athena can't open addresses on a private or local network."
  }
  if (raw.includes('unsupported content type')) {
    return "That link isn't a web page or text file."
  }
  if (raw.includes('page is too large')) {
    return 'That page is too large to import.'
  }
  if (raw.includes('no readable text')) {
    return 'No readable text was found. The page may need JavaScript or a login.'
  }
  if (raw.includes('invalid URL')) {
    return 'Enter a valid http or https link.'
  }
  if (raw.includes('knowledge index is loading')) {
    return 'The knowledge index is rebuilding. Please try again in a moment.'
  }
  return 'Failed to read the link. Please try again.'
}

// EventsOn returns its own unsubscribe function. Callers must invoke it on
// unmount to avoid leaking a listener across dialog opens (same discipline
// as the study:* events in lib/study.ts).
export function onIngestProgress(handler: (progress: IngestProgress) => void): () => void {
  return EventsOn('ingest:progress', (progress: IngestProgress) => handler(progress))
}

export function onFormattingProgress(handler: (progress: FormattingProgress) => void): () => void {
  return EventsOn('ingest:formatting-progress', (progress: FormattingProgress) => handler(progress))
}

export function onIngestDone(handler: (summary: IngestSummary) => void): () => void {
  return EventsOn('ingest:done', (summary: IngestSummary) => handler(summary))
}

export function onIngestError(handler: (message: string) => void): () => void {
  return EventsOn('ingest:error', (message: string) => handler(message))
}
