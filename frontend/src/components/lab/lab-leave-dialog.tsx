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
import { useTranslation } from 'react-i18next'

interface LabLeaveDialogProps {
  open: boolean
  onCancel: () => void
  onConfirm: () => void
}

// Confirms leaving the exam. The handlers belong to the caller: this dialog
// discards nothing by itself.
function LabLeaveDialog({ open, onCancel, onConfirm }: LabLeaveDialogProps) {
  const { t } = useTranslation()
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('exam.leaveTitle')}</AlertDialogTitle>
          <AlertDialogDescription>{t('exam.leaveDescription')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction
            // The action closes the dialog on click unless prevented, which
            // would also report a cancel. Closing is the caller's decision.
            onClick={(event) => {
              event.preventDefault()
              onConfirm()
            }}
          >
            {t('common.leave')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { LabLeaveDialog }
