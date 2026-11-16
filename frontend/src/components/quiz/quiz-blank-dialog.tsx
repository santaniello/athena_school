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

interface QuizBlankDialogProps {
  open: boolean
  // How many questions are still blank.
  blankCount: number
  onKeepAnswering: () => void
  onFinish: () => void
}

// Warns that blank questions count as wrong before the quiz is submitted.
function QuizBlankDialog({ open, blankCount, onKeepAnswering, onFinish }: QuizBlankDialogProps) {
  const { t } = useTranslation('quiz')
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && onKeepAnswering()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('take.blankTitle')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('take.blankDescription', { count: blankCount })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('take.keepAnswering')}</AlertDialogCancel>
          <AlertDialogAction onClick={onFinish}>{t('take.finishAnyway')}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { QuizBlankDialog }
