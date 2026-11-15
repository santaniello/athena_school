import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { ExamToStart } from '@/lib/exercise'
import { MAX_HINTS } from '@/lib/lab'
import { getPassScore } from '@/lib/passScore'
import { useTranslation } from 'react-i18next'

export type { ExamToStart }

interface StartExamDialogProps {
  // The challenge whose rules are shown; null keeps the dialog closed.
  challenge: ExamToStart | null
  onCancel: () => void
  onConfirm: (challenge: ExamToStart) => void
}

// The rules of the exam, shown before it starts. It never shows the scenario,
// requirements or rubric. See
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
export function StartExamDialog({ challenge, onCancel, onConfirm }: StartExamDialogProps) {
  return challenge ? (
    <StartExamContent challenge={challenge} onCancel={onCancel} onConfirm={onConfirm} />
  ) : null
}

function StartExamContent({
  challenge,
  onCancel,
  onConfirm,
}: Omit<StartExamDialogProps, 'challenge'> & { challenge: ExamToStart }) {
  const [passScore, setPassScore] = useState<number | null>(null)

  // Stryker disable BooleanLiteral,BlockStatement,ArrayDeclaration: the guard only keeps a late answer from updating an unmounted dialog, and the effect runs once on mount.
  useEffect(() => {
    let stale = false
    getPassScore()
      .then((score) => !stale && setPassScore(score))
      .catch(() => {
        // The rules are still true without the number; the exam can start.
      })
    return () => {
      stale = true
    }
  }, [])
  // Stryker restore BooleanLiteral,BlockStatement,ArrayDeclaration

  const { t } = useTranslation()
  const minutes = challenge.timeLimitMinutes
  return (
    <Dialog open onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{challenge.title}</DialogTitle>
          <DialogDescription>{t('startExam.rules')}</DialogDescription>
        </DialogHeader>
        <ul className="list-disc space-y-1 pl-5 text-sm text-foreground">
          <li>{minutes === 0 ? t('time.none') : t('startExam.timeLimit', { count: minutes })}</li>
          <li>{t('startExam.noPause')}</li>
          <li>{t('startExam.leaveDiscards')}</li>
          <li>{t('startExam.hints', { count: MAX_HINTS })}</li>
          {passScore !== null && <li>{t('startExam.passScore', { score: passScore })}</li>}
          <li>{t('startExam.afterSubmit')}</li>
        </ul>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          <Button type="button" onClick={() => onConfirm(challenge)}>
            {t('startExam.start')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
