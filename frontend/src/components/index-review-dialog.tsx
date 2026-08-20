import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import type { ChunkLoadIssue } from '@/lib/knowledge-index'

interface IndexReviewDialogProps {
  open: boolean
  issues: ChunkLoadIssue[]
  onClose: () => void
}

// Maps ListCurrent's stable reason codes (knowledge.ChunkIssue* in the Go
// domain) to plain-English copy — the raw code is never shown to the user.
const REASON_LABELS: Record<string, string> = {
  missing_item: 'The knowledge item this content belonged to no longer exists.',
  source_mismatch: "This content's source no longer matches its knowledge item.",
  topic_mismatch: "This content's topic no longer matches its knowledge item.",
  status_mismatch: "This content's status no longer matches its knowledge item.",
  stale_item: 'This knowledge item changed after this content was last indexed.',
  malformed_embedding: "This content's stored data is corrupted.",
  invalid_chunk_id: 'This content has an invalid identifier.',
  unknown_source: 'This content has an unrecognized source.',
  unknown_status: 'This content has an unrecognized status.',
  invalid_vector: "This content's stored data is invalid.",
}

function reasonLabel(reason: string): string {
  return REASON_LABELS[reason] ?? 'This content could not be indexed.'
}

// guidanceFor gives source-appropriate recovery guidance: imported notes can
// always be fixed by re-importing their folder; everything else has no
// self-service fix in this phase (Athena item reindexing ships in a later
// phase's consent-based backfill).
function guidanceFor(source: string): string {
  return source === 'imported_doc'
    ? 'Re-import the folder containing this file to fix this.'
    : 'This item is waiting on reindexing support in a future update.'
}

// Lists the chunks isolated from the last load/retry, identified only by
// safe fields (never a raw internal error), with a plain-English reason and
// source-appropriate recovery guidance.
function IndexReviewDialog({ open, issues, onClose }: IndexReviewDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Content needing attention</DialogTitle>
          <DialogDescription>
            These items were left out of local search. Local knowledge from before this remains
            available.
          </DialogDescription>
        </DialogHeader>

        <div className="thin-scroll max-h-80 space-y-2 overflow-y-auto">
          {issues.map((issue) => (
            <div key={issue.chunkId} className="rounded-lg border p-2 text-sm">
              <p className="truncate font-medium text-foreground">
                {issue.filePath || issue.chunkId}
              </p>
              <p className="text-muted-foreground">{reasonLabel(issue.reason)}</p>
              <p className="text-xs text-muted-foreground">{guidanceFor(issue.source)}</p>
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button onClick={onClose}>Close</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export { IndexReviewDialog }
