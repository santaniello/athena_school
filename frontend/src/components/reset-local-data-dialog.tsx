import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

interface ResetLocalDataDialogProps {
  open: boolean
  pending: boolean
  onCancel: () => void
  onConfirm: () => void
}

// Confirms the Settings screen's "Reset local data" action before it runs.
// See specs/phases/phase-01-desktop-mvp/13-reset-local-data.md — this is
// irreversible and does not touch the OpenRouter key or onboarding profile.
function ResetLocalDataDialog({ open, pending, onCancel, onConfirm }: ResetLocalDataDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && !pending && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Reset local data?</AlertDialogTitle>
          <AlertDialogDescription>
            This permanently deletes every study session, every folder you created, and your entire
            knowledge base. This action cannot be undone. Your OpenRouter key and profile are not
            affected.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            // AlertDialogAction is Radix's DialogPrimitive.Close under the
            // hood — it closes the dialog on click unconditionally unless
            // the click is prevented first. Reset stays open through
            // "pending" and a possible inline error, so closing is left
            // entirely to onCancel/the page reload after a real success.
            onClick={(event) => {
              event.preventDefault()
              onConfirm()
            }}
            disabled={pending}
          >
            {pending ? 'Resetting...' : 'Yes, reset local data'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { ResetLocalDataDialog }
