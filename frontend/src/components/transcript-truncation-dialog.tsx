import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface TranscriptTruncationDialogProps {
  open: boolean
  onConfirm: () => void
  onDecline: () => void
}

export function TranscriptTruncationDialog({
  open,
  onConfirm,
  onDecline,
}: TranscriptTruncationDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onDecline()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>This session is long</DialogTitle>
          <DialogDescription>
            Only the most recent complete messages will be considered for extraction. Continue?
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onDecline}>
            No
          </Button>
          <Button onClick={onConfirm}>Yes</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
