import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { QuizDifficultyBadge } from '@/components/quiz/quiz-difficulty-badge'
import { cn } from '@/lib/utils'
import { formatScore, scoreBand, type QuizAttemptResult, type QuizResultQuestion } from '@/lib/quiz'

const BAND_CLASSES = {
  low: 'text-red-700 dark:text-red-400',
  mid: 'text-amber-700 dark:text-amber-400',
  high: 'text-emerald-700 dark:text-emerald-400',
} as const

interface QuizResultViewProps {
  result: QuizAttemptResult
  // Offered only by a caller that can start the quiz again.
  onTakeAgain?: () => void
  // Offered only by a caller that has a screen to go back to.
  onBack?: () => void
}

// The graded result of one attempt: the grade out of 10, then every question
// with the user's choice, the correct option and the explanation. It is read
// only; the backend already decided what is right. See
// specs/phases/phase-03-architecture-lab/22-interactive-quiz-screen.md.
function QuizResultView({ result, onTakeAgain, onBack }: QuizResultViewProps) {
  const { t } = useTranslation('quiz')
  const [onlyMissed, setOnlyMissed] = useState(false)
  const { attempt, questions } = result
  const shown = onlyMissed ? questions.filter((question) => !question.correct) : questions

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <p
            data-band={scoreBand(attempt.score)}
            className={cn('text-3xl font-bold', BAND_CLASSES[scoreBand(attempt.score)])}
          >
            {t('result.grade', { score: formatScore(attempt.score) })}
          </p>
          <p className="text-sm text-muted-foreground">
            {t('result.correctOf', { correct: attempt.correctCount, total: attempt.total })}
          </p>
          <p className="flex items-center gap-2 text-sm text-foreground">
            <span>{result.sessionTitle}</span>
            <QuizDifficultyBadge difficulty={result.difficulty} />
          </p>
        </div>
        <div className="flex items-center gap-2">
          {onBack && (
            <Button type="button" variant="outline" onClick={onBack}>
              {t('result.back')}
            </Button>
          )}
          {onTakeAgain && (
            <Button type="button" onClick={onTakeAgain}>
              {t('result.takeAgain')}
            </Button>
          )}
        </div>
      </header>

      <label className="flex items-center gap-2 text-sm text-foreground">
        <input
          type="checkbox"
          checked={onlyMissed}
          onChange={(event) => setOnlyMissed(event.target.checked)}
        />
        {t('result.onlyMissed')}
      </label>

      {onlyMissed && shown.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('result.noneMissed')}</p>
      ) : (
        <ol className="space-y-3">
          {shown.map((question) => (
            <ResultQuestion key={question.id} question={question} />
          ))}
        </ol>
      )}
    </div>
  )
}

function ResultQuestion({ question }: { question: QuizResultQuestion }) {
  const { t } = useTranslation('quiz')
  const blank = question.selectedIndex === null
  return (
    <li
      aria-label={t('result.questionNumber', { number: question.position })}
      className="space-y-2 rounded-lg border border-border p-4"
    >
      <div className="flex items-start justify-between gap-3">
        <p className="text-sm font-semibold text-foreground">{question.prompt}</p>
        {question.correct ? (
          <Badge variant="beginner">{t('result.right')}</Badge>
        ) : blank ? (
          <Badge variant="muted">{t('result.blank')}</Badge>
        ) : (
          <Badge variant="advanced">{t('result.wrong')}</Badge>
        )}
      </div>
      <ul className="space-y-1">
        {question.options.map((option, index) => {
          const isCorrect = index === question.correctIndex
          const isChosen = index === question.selectedIndex
          return (
            <li
              key={option}
              className={cn(
                'flex items-center justify-between gap-3 rounded-md border px-3 py-1.5 text-sm',
                isCorrect
                  ? 'border-emerald-500/50 bg-emerald-500/10'
                  : isChosen
                    ? 'border-red-500/50 bg-red-500/10'
                    : 'border-transparent',
              )}
            >
              <span>{option}</span>
              <span className="flex gap-2 text-xs font-semibold text-muted-foreground">
                {isChosen && <span>{t('result.yourAnswer')}</span>}
                {isCorrect && <span>{t('result.correctAnswer')}</span>}
              </span>
            </li>
          )
        })}
      </ul>
      {blank && <p className="text-sm text-muted-foreground">{t('result.notAnswered')}</p>}
      <p className="text-sm text-muted-foreground">
        <span className="font-semibold">{t('result.explanation')}: </span>
        {question.explanation}
      </p>
    </li>
  )
}

export { QuizResultView }
