import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { QuizDifficultyBadge } from '@/components/quiz/quiz-difficulty-badge'
import { formatScore, type QuizSummary } from '@/lib/quiz'

interface SessionQuizCardProps {
  quiz: QuizSummary
  onStart: (quiz: QuizSummary) => void
  onHistory: (quiz: QuizSummary) => void
}

// The accepted quiz, kept under the tutor's message that offered it: the test,
// how it went last time and the buttons to take it or read its history. It
// never shows a question or an answer. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
function SessionQuizCard({ quiz, onStart, onHistory }: SessionQuizCardProps) {
  const { t } = useTranslation('quiz')
  const taken = quiz.attemptCount > 0

  return (
    <div className="flex flex-col items-start gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{quiz.sessionTitle}</p>
      <div className="flex flex-wrap items-center gap-2 text-muted-foreground">
        <QuizDifficultyBadge difficulty={quiz.difficulty} />
        <span>{t('catalog.questions', { count: quiz.questionCount })}</span>
        {taken && quiz.latestScore !== null ? (
          <>
            <span>{t('catalog.latestScore', { score: formatScore(quiz.latestScore) })}</span>
            <span>{t('catalog.attempts', { count: quiz.attemptCount })}</span>
          </>
        ) : (
          <span>{t('catalog.notTaken')}</span>
        )}
      </div>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => onStart(quiz)}>
          {taken ? t('sessionCard.startAgain') : t('sessionCard.start')}
        </Button>
        {taken && (
          <Button size="sm" variant="outline" onClick={() => onHistory(quiz)}>
            {t('common.history')}
          </Button>
        )}
      </div>
    </div>
  )
}

export { SessionQuizCard }
