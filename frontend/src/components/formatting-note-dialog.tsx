import { Loader2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

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
// cancel yet" stretch. See
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
function FormattingNoteDialog({ open }: FormattingNoteDialogProps) {
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
            Reformatting your note and preparing the chunks it will create.
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
