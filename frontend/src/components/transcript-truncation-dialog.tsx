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
          <DialogTitle>Esta sessão é longa</DialogTitle>
          <DialogDescription>
            Apenas as mensagens completas mais recentes serão consideradas na extração. Continuar?
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onDecline}>
            Não
          </Button>
          <Button onClick={onConfirm}>Sim</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
