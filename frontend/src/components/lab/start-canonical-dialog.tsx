import { useEffect, useRef, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  cancelChallengeGeneration,
  difficultyLabel,
  difficultyStateLabel,
  exerciseErrorCode,
  exerciseErrorMessage,
  generateCanonicalChallenge,
  type CanonicalProblem,
  type ChallengeDifficulty,
  type ExamToStart,
  type GeneratedChallenge,
} from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface StartCanonicalDialogProps {
  // The problem being started; null keeps the dialog closed.
  problem: CanonicalProblem | null
  // Preselected from the profile's experience level.
  defaultDifficulty: ChallengeDifficulty
  onClose: () => void
  // The challenge exists (generated now or reused): the caller shows the rules
  // of the exam, which starts it.
  onReady: (exam: ExamToStart) => void
  // The dialog was closed by the time the challenge arrived: it is stored, so
  // the caller only refreshes the list.
  onGeneratedAfterClose: () => void
}

// Asks for the difficulty and gets the challenge of the problem at it. The
// first time it generates it with one model call, with progress, cancel and
// retry; afterwards it reuses the stored one. See
// specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
export function StartCanonicalDialog({ problem, ...rest }: StartCanonicalDialogProps) {
  return problem ? <StartCanonicalContent problem={problem} {...rest} /> : null
}

function toExam(challenge: GeneratedChallenge): ExamToStart {
  return {
    id: challenge.id,
    title: challenge.title,
    difficulty: challenge.difficulty,
    timeLimitMinutes: challenge.timeLimitMinutes,
  }
}

function StartCanonicalContent({
  problem,
  defaultDifficulty,
  onClose,
  onReady,
  onGeneratedAfterClose,
}: Omit<StartCanonicalDialogProps, 'problem'> & { problem: CanonicalProblem }) {
  const { t } = useTranslation()
  const [difficulty, setDifficulty] = useState(defaultDifficulty)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // False once the dialog has closed, so a late answer is told apart from one
  // this instance is still waiting for.
  // Stryker disable next-line BooleanLiteral: the mount effect below sets it before any answer can arrive.
  const mountedRef = useRef(false)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  const chosen = problem.difficulties.find((state) => state.difficulty === difficulty)

  async function handleStart() {
    setError(null)
    setGenerating(true)
    try {
      const challenge = await generateCanonicalChallenge(problem.id, difficulty)
      if (mountedRef.current) onReady(toExam(challenge))
      else onGeneratedAfterClose()
    } catch (err) {
      // A cancelled generation is the user's own doing: back to the choice, quietly.
      if (exerciseErrorCode(err) !== 'cancelled') setError(exerciseErrorMessage(err))
      setGenerating(false)
    }
  }

  // Dismissing the dialog (Escape, the close button) must not leave a
  // generation running that nobody is waiting for.
  function handleDismiss() {
    if (generating) void cancelChallengeGeneration()
    onClose()
  }

  return (
    <Dialog open onOpenChange={(nextOpen) => !nextOpen && handleDismiss()}>
      <DialogContent closeLabel={t('common.close')} className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('canonical.startTitle', { name: problem.name })}</DialogTitle>
          <DialogDescription>{t('canonical.startDescription')}</DialogDescription>
        </DialogHeader>

        <div role="radiogroup" aria-label={t('canonical.difficulty')} className="space-y-1">
          {problem.difficulties.map((state) => (
            // A native radio gives one tab stop and the arrow keys for free.
            <label
              key={state.difficulty}
              className="flex w-full cursor-pointer items-center justify-between rounded-lg border border-border px-3 py-2 text-sm transition-colors hover:bg-accent has-[:checked]:border-primary has-[:checked]:bg-primary/10 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
            >
              <input
                type="radio"
                name="canonical-difficulty"
                className="sr-only"
                checked={state.difficulty === difficulty}
                disabled={generating}
                onChange={() => setDifficulty(state.difficulty)}
              />
              <span className="font-medium text-foreground">
                {difficultyLabel(state.difficulty)}
              </span>
              <span className="text-muted-foreground">{difficultyStateLabel(state)}</span>
            </label>
          ))}
        </div>

        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {generating && chosen && !chosen.generated && (
          <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            {t('canonical.generating')}
          </p>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={generating ? () => void cancelChallengeGeneration() : onClose}
          >
            {t('common.cancel')}
          </Button>
          {!generating && (
            <Button type="button" onClick={() => void handleStart()}>
              {error ? t('common.tryAgain') : t('canonical.submit')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
