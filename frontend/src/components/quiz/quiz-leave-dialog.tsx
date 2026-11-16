import { useTranslation } from 'react-i18next'
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

interface QuizLeaveDialogProps {
  open: boolean
  onCancel: () => void
  onConfirm: () => void
}

// Confirms leaving a quiz in progress. The answers are not stored, so leaving
// loses them; the handlers belong to the caller.
function QuizLeaveDialog({ open, onCancel, onConfirm }: QuizLeaveDialogProps) {
  const { t } = useTranslation('quiz')
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('take.leaveTitle')}</AlertDialogTitle>
          <AlertDialogDescription>{t('take.leaveDescription')}</AlertDialogDescription>
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

export { QuizLeaveDialog }
