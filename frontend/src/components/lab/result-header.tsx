import { DifficultyBadge } from '@/components/difficulty-badge'
import { formatCountdown } from '@/lib/lab'
import { formatSessionDate, type AttemptResult } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface ResultHeaderProps {
  result: AttemptResult
}

// The title of a finished attempt and how it went: when, how long, how it
// ended and how many hints it used.
export function ResultHeader({ result }: ResultHeaderProps) {
  const { t } = useTranslation()
  const { challenge } = result
  return (
    <header className="flex flex-col gap-1">
      <h2 className="text-2xl font-semibold text-foreground">{challenge.title}</h2>
      <p className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
        <span>{challenge.topic}</span>
        <DifficultyBadge difficulty={challenge.difficulty} />
      </p>
      <p className="text-sm text-muted-foreground">
        {formatSessionDate(result.finishedAt)} ·{' '}
        {t('result.timeSpent', { time: formatCountdown(result.timeSpentSeconds * 1000) })} ·{' '}
        {t(`result.submitReason.${result.submitReason}`)} ·{' '}
        {t('result.hints', { used: result.hintsUsed, max: result.maxHints })}
      </p>
    </header>
  )
}
