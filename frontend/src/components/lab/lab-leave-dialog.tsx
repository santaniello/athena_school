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

interface LabLeaveDialogProps {
  open: boolean
  onCancel: () => void
  onConfirm: () => void
}

// Confirms leaving the exam. The handlers belong to the caller: this dialog
// discards nothing by itself.
function LabLeaveDialog({ open, onCancel, onConfirm }: LabLeaveDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Leave the exam?</AlertDialogTitle>
          <AlertDialogDescription>
            Leaving discards this exam. It will not be graded.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            // The action closes the dialog on click unless prevented, which
            // would also report a cancel. Closing is the caller's decision.
            onClick={(event) => {
              event.preventDefault()
              onConfirm()
            }}
          >
            Leave
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { LabLeaveDialog }
