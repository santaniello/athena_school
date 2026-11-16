import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { QuizDifficultyBadge } from '@/components/quiz/quiz-difficulty-badge'
import { QuizLeaveDialog } from '@/components/quiz/quiz-leave-dialog'
import { QuizResultView } from '@/components/quiz/quiz-result-view'
import { useQuizResult } from '@/hooks/use-quiz-result'
import { useQuizSession } from '@/hooks/use-quiz-session'
import { type QuizAttemptSummary, type QuizReturn } from '@/lib/quiz'
import { cn } from '@/lib/utils'

interface QuizTakeScreenProps {
  quizId: string
  // Where the quiz goes back to when it ends; it only names the button.
  returnTo: QuizReturn
  // The shell wants to navigate away; the leave question is asked here.
  leaveRequested: boolean
  onLeaveCancelled: () => void
  // The user is done: left the quiz or went back to the catalog after it.
  onExit: () => void
}

// A quiz being taken, full screen: one question at a time with the answers
// kept in memory until Finish. Once graded, the same screen shows the result,
// and Take again starts a fresh run. See
// specs/phases/phase-03-architecture-lab/22-interactive-quiz-screen.md.
function QuizTakeScreen({
  quizId,
  returnTo,
  leaveRequested,
  onLeaveCancelled,
  onExit,
}: QuizTakeScreenProps) {
  const [attempt, setAttempt] = useState<QuizAttemptSummary | null>(null)
  // Each run is a fresh set of answers: Take again remounts the taking stage.
  const [run, setRun] = useState(0)

  // A graded quiz has nothing left to lose, so a navigation request is granted.
  useEffect(() => {
    if (attempt && leaveRequested) onExit()
  }, [attempt, leaveRequested, onExit])

  if (attempt) {
    return (
      <ResultStage
        attemptId={attempt.id}
        backTo={returnTo.kind}
        onTakeAgain={() => {
          setAttempt(null)
          setRun((current) => current + 1)
        }}
        onBack={onExit}
      />
    )
  }
  return (
    <TakingStage
      key={run}
      quizId={quizId}
      returnTo={returnTo}
      leaveRequested={leaveRequested}
      onLeaveCancelled={onLeaveCancelled}
      onExit={onExit}
      onGraded={setAttempt}
    />
  )
}

interface TakingStageProps extends QuizTakeScreenProps {
  onGraded: (attempt: QuizAttemptSummary) => void
}

function TakingStage({
  quizId,
  returnTo,
  leaveRequested,
  onLeaveCancelled,
  onExit,
  onGraded,
}: TakingStageProps) {
  const { t } = useTranslation('quiz')
  const session = useQuizSession(quizId)
  const [leaving, setLeaving] = useState(false)

  // Until the quiz has loaded there is nothing to lose, so a navigation
  // request is granted instead of asking.
  const ready = session.state === 'ready'
  useEffect(() => {
    if (leaveRequested && !ready) onExit()
  }, [leaveRequested, ready, onExit])

  async function submit() {
    const graded = await session.submit()
    if (graded) onGraded(graded)
  }

  if (session.state === 'loading') {
    return <p className="text-sm text-muted-foreground">{t('take.loading')}</p>
  }
  if (session.state === 'failed') {
    return (
      <div role="alert" className="flex items-center gap-3 text-sm">
        <span className="text-destructive">{t('catalog.openFailed')}</span>
        <Button type="button" variant="outline" size="sm" onClick={session.reload}>
          {t('common.retry')}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onExit}>
          {t(returnTo.kind === 'study' ? 'result.backToSession' : 'result.back')}
        </Button>
      </div>
    )
  }

  const { quiz, question, index, selections } = session
  if (!quiz || !question) return null
  const chosen = selections[index].selectedIndex
  const last = index === quiz.questions.length - 1

  return (
    <div className="flex min-h-0 mx-auto w-full max-w-3xl flex-col gap-5">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <p className="flex items-center gap-2 text-sm font-semibold text-foreground">
            <span>{quiz.sessionTitle}</span>
            <QuizDifficultyBadge difficulty={quiz.difficulty} />
          </p>
          <p className="text-sm text-muted-foreground">
            {t('take.progress', { current: index + 1, total: quiz.questions.length })}
          </p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={() => setLeaving(true)}>
          {t('common.leave')}
        </Button>
      </header>

      <nav aria-label={t('take.stripLabel')} className="flex flex-wrap gap-1.5">
        {selections.map((selection, position) => (
          <Button
            key={selection.questionId}
            type="button"
            size="sm"
            variant={position === index ? 'default' : 'outline'}
            aria-current={position === index ? 'step' : undefined}
            aria-label={t(
              selection.selectedIndex === null
                ? 'take.questionButton'
                : 'take.questionButtonAnswered',
              { number: position + 1 },
            )}
            className={cn(
              selection.selectedIndex !== null && position !== index && 'border-primary/60',
            )}
            onClick={() => session.goTo(position)}
          >
            {position + 1}
          </Button>
        ))}
      </nav>

      <section className="space-y-3">
        <p className="text-base font-semibold text-foreground">{question.prompt}</p>
        <div role="radiogroup" aria-label={t('take.optionsLabel')} className="space-y-2">
          {question.options.map((option, optionIndex) => (
            // A native radio gives one tab stop and the arrow keys for free.
            <label
              key={optionIndex}
              className="flex cursor-pointer items-center gap-3 rounded-lg border border-border px-3 py-2 text-sm transition-colors hover:bg-accent has-[:checked]:border-primary has-[:checked]:bg-primary/10 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
            >
              <input
                type="radio"
                name={`question-${question.id}`}
                checked={chosen === optionIndex}
                disabled={session.submitting}
                onChange={() => session.answer(optionIndex)}
              />
              <span>{option}</span>
            </label>
          ))}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={chosen === null || session.submitting}
          onClick={() => session.answer(null)}
        >
          {t('take.clear')}
        </Button>
      </section>

      {session.submitError && (
        <p role="alert" className="text-sm text-destructive">
          {session.submitError}
        </p>
      )}
      {session.submitting && (
        <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" aria-hidden="true" />
          {t('take.submitting')}
        </p>
      )}

      <footer className="flex items-center justify-between gap-3">
        <div className="flex gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={index === 0 || session.submitting}
            onClick={session.previous}
          >
            {t('take.previous')}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={last || session.submitting}
            onClick={session.next}
          >
            {t('take.next')}
          </Button>
        </div>
        {session.blankCount === 0 && (
          <Button type="button" disabled={session.submitting} onClick={() => void submit()}>
            {t('take.finish')}
          </Button>
        )}
      </footer>

      <QuizLeaveDialog
        open={leaving || leaveRequested}
        onCancel={() => {
          setLeaving(false)
          onLeaveCancelled()
        }}
        onConfirm={onExit}
      />
    </div>
  )
}

function ResultStage({
  attemptId,
  backTo,
  onTakeAgain,
  onBack,
}: {
  attemptId: string
  backTo: QuizReturn['kind']
  onTakeAgain: () => void
  onBack: () => void
}) {
  const { t } = useTranslation('quiz')
  const { state, result, reload } = useQuizResult(attemptId)
  return (
    <div className="thin-scroll min-h-0 mx-auto w-full max-w-3xl overflow-y-auto">
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
      {result && (
        <QuizResultView result={result} onTakeAgain={onTakeAgain} onBack={onBack} backTo={backTo} />
      )}
    </div>
  )
}

export default QuizTakeScreen
