import { useState } from 'react'
import { ChevronDown } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'
import type { ChunkPreview, ImportPreview } from '@/lib/ingest'

// statsLabel renders preview.stats as a single scannable line — every
// number here is computed deterministically by the application (never
// narrated by the model itself), per
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md
// decision 8.
function statsLabel(stats: ImportPreview['stats']): string {
  return (
    `${stats.headingsBefore} → ${stats.headingsAfter} headings · ` +
    `${stats.paragraphsBefore} → ${stats.paragraphsAfter} paragraphs`
  )
}

interface ImportReviewDialogProps {
  open: boolean
  // null while no file has been picked, or while its preview is still
  // loading — the dialog renders nothing in that case, regardless of open.
  preview: ImportPreview | null
  onCancel: () => void
  // Receives exactly preview.formattedContent — never anything the caller
  // itself decides — so what was reviewed here is what confirmImport later
  // persists (spec 2.20 decision 4).
  onApprove: (formattedContent: string) => void
}

type ReviewTab = 'formatted' | 'chunks'

// A chunk card collapses to this many characters before "Read full chunk"
// is needed — long enough to judge the chunk at a glance, short enough
// that a list of them stays scannable.
const CHUNK_PREVIEW_LENGTH = 160

function truncate(text: string, length: number): string {
  if (text.length <= length) return text
  return `${text.slice(0, length)}…`
}

interface ChunkCardProps {
  index: number
  chunk: ChunkPreview
  expanded: boolean
  onToggle: () => void
}

function chunkCountLabel(count: number): string {
  return `${count} chunk${count === 1 ? '' : 's'}`
}

function ChunkCard({ index, chunk, expanded, onToggle }: ChunkCardProps) {
  const isTruncated = chunk.content.length > CHUNK_PREVIEW_LENGTH
  return (
    <div
      className={cn(
        'rounded-lg border border-border p-3',
        expanded && 'border-primary/40 bg-primary/5',
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs font-semibold text-foreground">
          <span className="mr-1.5 rounded-full bg-primary px-1.5 py-0.5 text-[10px] font-bold text-primary-foreground">
            {index + 1}
          </span>
          {chunk.heading || 'Untitled passage'}
        </p>
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {chunk.charCount} chars · ~{chunk.estimatedTokens} tokens
        </span>
      </div>
      <p className="mt-1.5 text-xs leading-relaxed text-foreground">
        {expanded ? chunk.content : truncate(chunk.content, CHUNK_PREVIEW_LENGTH)}
      </p>
      {isTruncated && (
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={onToggle}
          className="mt-2 h-6 gap-1 rounded-full px-2.5 text-[11px]"
        >
          <ChevronDown
            className={cn('size-2.5 transition-transform', expanded && 'rotate-180')}
            aria-hidden="true"
          />
          {expanded ? 'Show preview' : 'Read full chunk'}
        </Button>
      )}
    </div>
  )
}

// The gate every file now passes through before anything is embedded or
// persisted: the reformatted note and the chunks it will produce, approved
// or cancelled by the user. See
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
function ImportReviewDialog({ open, preview, onCancel, onApprove }: ImportReviewDialogProps) {
  const [tab, setTab] = useState<ReviewTab>('formatted')
  const [expandedChunk, setExpandedChunk] = useState<number | null>(null)

  if (!preview) return null

  function toggleChunk(index: number) {
    setExpandedChunk((current) => (current === index ? null : index))
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onCancel()}>
      <DialogContent className="flex max-h-[85vh] flex-col sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Review import</DialogTitle>
          <DialogDescription>
            We reformatted <strong className="text-foreground">“{preview.displayPath}”</strong> to
            improve search quality. Review the result and the chunks it will create, then approve or
            cancel the import.
          </DialogDescription>
        </DialogHeader>

        {preview.formattingFailed && (
          <Alert>
            <AlertDescription>
              {preview.failedSections.length > 0 ? (
                <>
                  Formatting couldn't be applied to{' '}
                  {preview.failedSections.length === 1 ? 'one section' : 'some sections'} — showing
                  the original text there. You can still import it as-is.
                  <ul className="mt-1.5 list-disc space-y-0.5 pl-4 text-[11px]">
                    {preview.failedSections.map((section, index) => (
                      <li key={index}>
                        <strong>{section.heading || 'Untitled section'}</strong>: {section.reason}
                      </li>
                    ))}
                  </ul>
                </>
              ) : (
                'Formatting is unavailable right now — showing your original text. You can still import it as-is.'
              )}
            </AlertDescription>
          </Alert>
        )}

        {preview.structureWarnings.length > 0 && (
          <Alert>
            <AlertDescription>
              Some sections may still group distinct items into one chunk after formatting:
              <ul className="mt-1.5 list-disc space-y-0.5 pl-4 text-[11px]">
                {preview.structureWarnings.map((warning, index) => (
                  <li key={index}>
                    <strong>{warning.heading || 'Untitled section'}</strong>: {warning.message}
                  </li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        )}

        {!preview.formattingFailed && (
          <p className="text-[11px] text-muted-foreground">{statsLabel(preview.stats)}</p>
        )}

        <div className="flex gap-2">
          <Button
            type="button"
            size="sm"
            variant={tab === 'formatted' ? 'default' : 'outline'}
            onClick={() => setTab('formatted')}
            className="rounded-full"
          >
            Formatted note
          </Button>
          <Button
            type="button"
            size="sm"
            variant={tab === 'chunks' ? 'default' : 'outline'}
            onClick={() => setTab('chunks')}
            className="rounded-full"
          >
            Chunks ({preview.chunks.length})
          </Button>
        </div>

        <div className="thin-scroll min-h-0 flex-1 overflow-y-auto">
          {tab === 'formatted' &&
            (preview.formattingFailed ? (
              <div className="rounded-lg border border-border p-3 text-xs leading-relaxed whitespace-pre-wrap text-foreground">
                {preview.formattedContent}
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <p className="mb-1.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
                    Original
                  </p>
                  <div className="h-64 overflow-y-auto rounded-lg border border-border p-2.5 text-[11px] leading-relaxed whitespace-pre-wrap text-muted-foreground">
                    {preview.originalContent}
                  </div>
                </div>
                <div>
                  <p className="mb-1.5 text-[11px] font-semibold tracking-wide text-primary uppercase">
                    Formatted
                  </p>
                  <div className="h-64 overflow-y-auto rounded-lg border border-primary/30 bg-primary/5 p-2.5 text-xs leading-relaxed whitespace-pre-wrap text-foreground">
                    {preview.formattedContent}
                  </div>
                </div>
              </div>
            ))}

          {tab === 'chunks' && (
            <div className="flex flex-col gap-2">
              <p className="text-xs text-muted-foreground">
                {chunkCountLabel(preview.chunks.length)} will be created from this document — each
                is the passage retrieved later in chat. Its embedding also includes the section
                heading shown above it, for context.
              </p>
              {preview.chunks.map((chunk, index) => (
                <ChunkCard
                  key={index}
                  index={index}
                  chunk={chunk}
                  expanded={expandedChunk === index}
                  onToggle={() => toggleChunk(index)}
                />
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" onClick={() => onApprove(preview.formattedContent)}>
            Approve &amp; import
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export { ImportReviewDialog }
