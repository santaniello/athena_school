import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { onFormattingProgress, type FormattingProgress } from '@/lib/ingest'

interface FormattingNoteDialogProps {
  open: boolean
}

// Shown while previewImport is in flight — the moment between clicking Add
// and the Review dialog opening (or a silent skip reloading the list)
// that, without this, left the user staring at the unchanged panel with
// only a small inline status line to notice. Closes itself the instant the
// caller flips open to false, which happens in the very same state update
// that either opens ImportReviewDialog or finishes the skip/error path —
// never dismissible on its own, same as IngestProgressDialog's "nothing to
// cancel yet" stretch. Subscribes to its own "ingest:formatting-progress"
// ticks (the same self-contained pattern IngestProgressDialog uses for
// "ingest:progress") now that PreviewImport formats a document section by
// section instead of in one call, showing "Formatting section X of Y"
// once more than one batch exists — a single-batch note never shows a
// count, since "1 of 1" tells the user nothing a plain spinner doesn't.
// See specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
function FormattingNoteDialog({ open }: FormattingNoteDialogProps) {
  const [progress, setProgress] = useState<FormattingProgress | null>(null)

  useEffect(() => {
    if (!open) return

    const unsubscribe = onFormattingProgress(setProgress)
    return () => {
      unsubscribe()
      setProgress(null)
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={() => {}}>
      <DialogContent
        showCloseButton={false}
        className="sm:max-w-sm"
        onEscapeKeyDown={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>Formatting note…</DialogTitle>
          <DialogDescription>
            {progress && progress.sectionsTotal > 1
              ? `Formatting section ${progress.sectionsProcessed} of ${progress.sectionsTotal}…`
              : 'Reformatting your note and preparing the chunks it will create.'}
          </DialogDescription>
        </DialogHeader>
        <div className="flex items-center justify-center py-2">
          <Loader2 className="size-6 animate-spin text-primary" aria-hidden="true" />
        </div>
      </DialogContent>
    </Dialog>
  )
}

export { FormattingNoteDialog }
