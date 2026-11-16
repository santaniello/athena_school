import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { QuizResultView } from '@/components/quiz/quiz-result-view'
import { useQuizResult } from '@/hooks/use-quiz-result'
import { cn } from '@/lib/utils'
import { formatQuizDate, formatScore, listQuizAttempts, type QuizAttemptSummary } from '@/lib/quiz'

export interface HistoryQuiz {
  id: string
  title: string
}

interface QuizHistoryDialogProps {
  // The quiz whose attempts are listed; null keeps the dialog closed.
  quiz: HistoryQuiz | null
  onClose: () => void
}

// The attempts of a quiz, newest first. Choosing one opens its result in the
// same dialog; nothing here changes or deletes an attempt.
export function QuizHistoryDialog({ quiz, onClose }: QuizHistoryDialogProps) {
  return quiz ? <HistoryContent quiz={quiz} onClose={onClose} /> : null
}

function HistoryContent({ quiz, onClose }: { quiz: HistoryQuiz; onClose: () => void }) {
  const { t } = useTranslation('quiz')
  const [attempts, setAttempts] = useState<QuizAttemptSummary[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)

  useEffect(() => {
    let stale = false
    listQuizAttempts(quiz.id)
      .then((loaded) => !stale && setAttempts(loaded))
      .catch(() => !stale && setFailed(true))
    return () => {
      stale = true
    }
  }, [quiz.id])

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        closeLabel={t('common.close')}
        className={cn('max-h-[90vh] overflow-y-auto', selected ? 'sm:max-w-3xl' : 'sm:max-w-md')}
      >
        <DialogHeader>
          <DialogTitle>{quiz.title}</DialogTitle>
          <DialogDescription>{t('history.description')}</DialogDescription>
        </DialogHeader>
        {selected ? (
          <SelectedResult attemptId={selected} onBack={() => setSelected(null)} />
        ) : failed ? (
          <p role="alert" className="text-sm text-destructive">
            {t('history.failed')}
          </p>
        ) : attempts === null ? (
          <p className="text-sm text-muted-foreground">{t('history.loading')}</p>
        ) : attempts.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('history.empty')}</p>
        ) : (
          <ul className="divide-y divide-border rounded-lg border border-border">
            {attempts.map((attempt) => (
              <li key={attempt.id} className="flex items-center justify-between gap-3 px-3 py-2">
                <div className="text-sm">
                  <p className="font-semibold text-foreground">
                    {t('history.item', {
                      score: formatScore(attempt.score),
                      correct: attempt.correctCount,
                      total: attempt.total,
                    })}
                  </p>
                  <p className="text-muted-foreground">{formatQuizDate(attempt.finishedAt)}</p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setSelected(attempt.id)}
                >
                  {t('history.open')}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </DialogContent>
    </Dialog>
  )
}

function SelectedResult({ attemptId, onBack }: { attemptId: string; onBack: () => void }) {
  const { t } = useTranslation('quiz')
  const { state, result, reload } = useQuizResult(attemptId)
  return (
    <div className="flex flex-col gap-3">
      <Button type="button" variant="outline" size="sm" className="self-start" onClick={onBack}>
        {t('history.back')}
      </Button>
      {state === 'loading' && (
        <p className="text-sm text-muted-foreground">{t('result.loading')}</p>
      )}
      {state === 'failed' && (
        <div role="alert" className="flex items-center gap-3 text-sm">
          <span className="text-destructive">{t('result.loadFailed')}</span>
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            {t('common.retry')}
          </Button>
        </div>
      )}
      {result && <QuizResultView result={result} />}
    </div>
  )
}
