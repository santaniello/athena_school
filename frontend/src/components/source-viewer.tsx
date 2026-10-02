import { useEffect, useRef } from 'react'
import { ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useSourceDocument } from '@/hooks/use-source-document'

interface SourceViewerProps {
  sessionId: string
  itemId: string
  onBack: () => void
  // The chunk a citation click asked to see, scrolled into view and briefly
  // highlighted once the document loads. Undefined renders the document
  // with no highlight, same as a plain Sources-panel row click.
  chunkId?: string
  // An opaque value that changes on every open request, even a repeat click
  // on the exact same citation (sessionId/itemId/chunkId all unchanged in
  // that case). Forces the scroll-and-highlight effect to re-run and the
  // highlighted segment to remount (restarting its fade animation) on every
  // open, not just the first one for a given chunkId.
  reopenToken?: number
}

// A document imported before spec 2.18 (or whose text was otherwise lost)
// has no stored text to show — GetSourceDocument reports this distinctly
// from "does not exist at all" so the viewer can explain it, rather than
// showing a generic error. Matched by message text, the same convention
// every other ingest binding error already uses on the frontend (see
// study-sources-panel.tsx's removeError handling).
const TEXT_UNAVAILABLE_MESSAGE = 'source text unavailable'
// itemId not existing at all, or having just been removed from this
// session, are reported the same way by the backend (ErrSourceNotFound) —
// so is this viewer's copy for both.
const NOT_FOUND_MESSAGE = 'source not found'

// The Sources panel's document reader: a session's imported document as
// plain text, line breaks preserved (rendering Markdown here is a later
// refinement — see spec 2.18). Segments are rendered individually, each
// tagged with its own chunkId, laying the groundwork for a cited passage
// to be highlighted and scrolled into view — not wired up until the
// citations feature lands.
function SourceViewer({ sessionId, itemId, onBack, chunkId, reopenToken }: SourceViewerProps) {
  const { document, loading, error, reload } = useSourceDocument(sessionId, itemId)
  const segmentsRef = useRef<HTMLDivElement>(null)

  // Scrolls the cited passage into view once the document (and its
  // segments) are actually on the page. Segments already carry their own
  // chunkId (data-chunk-id, from PR 1) — this just needed to find and
  // scroll to the one that matches.
  useEffect(() => {
    // Stryker disable next-line ConditionalExpression,LogicalOperator: this
    // guard is a redundant optimization, not a correctness gap — the
    // ref/DOM guarantee it short-circuits is already structurally enforced
    // below regardless: segmentsRef only attaches to the div this effect
    // targets once `document` is truthy (it's rendered in that same branch),
    // and no segment's data-chunk-id is ever the empty/falsy value chunkId
    // would be compared against when absent. Removing this line changes no
    // observable outcome in any reachable state.
    if (!chunkId || !document) return
    // Stryker disable next-line OptionalChaining: unreachable without a
    // ref — see the guard's own comment above; the div this ref attaches to
    // is only ever rendered once `document` is truthy, and effects run
    // after commit, so the ref is always populated by the time this runs.
    const candidates = segmentsRef.current?.querySelectorAll<HTMLElement>('[data-chunk-id]')
    const target = candidates && Array.from(candidates).find((el) => el.dataset.chunkId === chunkId)
    target?.scrollIntoView({ block: 'center' })
  }, [document, chunkId, reopenToken])

  return (
    <div className="flex h-full w-full flex-col overflow-hidden bg-[oklch(0.115_0.014_50)]">
      <div className="flex flex-col gap-2 border-b border-border p-3">
        <Button
          size="sm"
          variant="ghost"
          onClick={onBack}
          className="-ml-2 w-fit gap-1 text-muted-foreground"
        >
          <ArrowLeft className="size-3.5" aria-hidden="true" />
          Sources
        </Button>
        {document && (
          <div className="min-w-0">
            <h2 className="truncate font-heading text-base font-bold text-foreground">
              {document.title}
            </h2>
            <p className="truncate text-[11px] text-muted-foreground">{document.path}</p>
          </div>
        )}
      </div>

      <div className="thin-scroll min-h-0 flex-1 overflow-y-auto p-3">
        {loading ? (
          <p className="p-4 text-center text-xs text-muted-foreground">Loading document…</p>
        ) : error?.includes(TEXT_UNAVAILABLE_MESSAGE) ? (
          <div className="flex flex-col items-center gap-2 p-4 text-center">
            <p className="text-xs text-foreground">Re-import this document to open it here.</p>
          </div>
        ) : error?.includes(NOT_FOUND_MESSAGE) ? (
          <div className="flex flex-col items-center gap-2 p-4 text-center">
            <p className="text-xs text-muted-foreground">
              This source was removed from the session.
            </p>
          </div>
        ) : error ? (
          <div className="flex flex-col items-center gap-2 p-4 text-center">
            <p className="text-xs text-destructive">{error}</p>
            <Button size="sm" variant="outline" onClick={reload}>
              Retry
            </Button>
          </div>
        ) : (
          <div
            ref={segmentsRef}
            className="whitespace-pre-wrap text-xs leading-relaxed text-foreground"
          >
            {/* Stryker disable next-line OptionalChaining: this branch only
                renders once loading is false and error is falsy, which only
                happens after a successful load — document is guaranteed
                set here, so the "?." is defensive, not itself a gap. */}
            {document?.segments.map((segment, index) => {
              const highlighted = Boolean(segment.chunkId) && segment.chunkId === chunkId
              return (
                <span
                  // Keying the highlighted segment off reopenToken too forces
                  // React to remount it (not just re-render) on every open,
                  // restarting the CSS fade animation even when it's the same
                  // segment as last time.
                  key={highlighted ? `${index}-${reopenToken}` : index}
                  data-chunk-id={segment.chunkId || undefined}
                  className={highlighted ? 'citation-highlight' : undefined}
                >
                  {segment.text}
                </span>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

export { SourceViewer }
