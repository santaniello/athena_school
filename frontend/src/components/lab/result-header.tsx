import { DifficultyBadge } from '@/components/difficulty-badge'
import { formatCountdown } from '@/lib/lab'
import { formatSessionDate, type AttemptResult } from '@/lib/exercise'

interface ResultHeaderProps {
  result: AttemptResult
}

const SUBMIT_REASON_LABELS: Record<AttemptResult['submitReason'], string> = {
  USER: 'Finished by you',
  TIME_EXPIRED: 'Ended by time',
}

// The title of a finished attempt and how it went: when, how long, how it
// ended and how many hints it used.
export function ResultHeader({ result }: ResultHeaderProps) {
  const { challenge } = result
  return (
    <header className="flex flex-col gap-1">
      <h2 className="text-2xl font-semibold text-foreground">{challenge.title}</h2>
      <p className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        <span>{challenge.topic}</span>
        <DifficultyBadge difficulty={challenge.difficulty} />
      </p>
      <p className="text-sm text-muted-foreground">
        {formatSessionDate(result.finishedAt)} · Time spent{' '}
        {formatCountdown(result.timeSpentSeconds * 1000)} ·{' '}
        {SUBMIT_REASON_LABELS[result.submitReason]} · Hints {result.hintsUsed} of {result.maxHints}
      </p>
    </header>
  )
}
