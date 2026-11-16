import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { QuizDifficultyBadge } from '@/components/quiz/quiz-difficulty-badge'
import { formatSessionDate } from '@/lib/exercise'
import { formatScore, type QuizSummary } from '@/lib/quiz'
import { cn } from '@/lib/utils'

interface QuizRowProps {
  quiz: QuizSummary
  // True for the quiz that was just generated.
  highlighted: boolean
  onTake: (quiz: QuizSummary) => void
  onHistory: (quiz: QuizSummary) => void
  onDelete: (quiz: QuizSummary) => void
}

// One catalog row: the session the quiz is about, how big and hard it is and
// how the last attempt went. It never shows a question.
function QuizRow({ quiz, highlighted, onTake, onHistory, onDelete }: QuizRowProps) {
  const { t } = useTranslation('quiz')
  const taken = quiz.attemptCount > 0

  return (
    <li
      aria-label={quiz.sessionTitle}
      aria-current={highlighted ? 'true' : undefined}
      className={cn(
        'flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3',
        highlighted && 'bg-primary/10 ring-1 ring-primary/40',
      )}
    >
      <div className="min-w-0 flex-1 basis-48">
        <p className="truncate text-sm font-semibold text-foreground">{quiz.sessionTitle}</p>
        <p className="flex flex-wrap gap-x-2 text-sm text-muted-foreground">
          <span>{t('catalog.questions', { count: quiz.questionCount })}</span>
          <span>{formatSessionDate(quiz.createdAt)}</span>
          {taken && quiz.latestScore !== null ? (
            <>
              <span>{t('catalog.latestScore', { score: formatScore(quiz.latestScore) })}</span>
              <span>{t('catalog.attempts', { count: quiz.attemptCount })}</span>
            </>
          ) : (
            <span>{t('catalog.notTaken')}</span>
          )}
        </p>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-3">
        <QuizDifficultyBadge difficulty={quiz.difficulty} />
        {taken && (
          <Button type="button" variant="outline" size="sm" onClick={() => onHistory(quiz)}>
            {t('common.history')}
          </Button>
        )}
        <Button type="button" variant="outline" size="sm" onClick={() => onTake(quiz)}>
          {taken ? t('catalog.takeAgain') : t('catalog.take')}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => onDelete(quiz)}>
          {t('catalog.delete')}
        </Button>
      </div>
    </li>
  )
}

export { QuizRow }
