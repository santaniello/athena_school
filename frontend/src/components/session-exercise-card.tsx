import { Badge } from '@/components/ui/badge'
import { DifficultyBadge } from '@/components/difficulty-badge'
import { Button } from '@/components/ui/button'
import {
  formatTimeLimit,
  kindLabel,
  statusLabel,
  type SessionExerciseCard as Card,
} from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface SessionExerciseCardProps {
  card: Card
  onStart: (card: Card) => void
  onHistory: (card: Card) => void
}

// The accepted exercise, kept under the tutor's message that offered it: the
// challenge, how the user is doing on it and the buttons to start it or read
// its history. It never shows the scenario, requirements or rubric. See
// specs/phases/phase-03-architecture-lab/17-session-exercises-ui.md.
function SessionExerciseCard({ card, onStart, onHistory }: SessionExerciseCardProps) {
  const { t } = useTranslation()
  const status = statusLabel(card.status)
  const { latest } = card

  return (
    <div className="flex flex-col items-start gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{card.title}</p>
      <div className="flex flex-wrap items-center gap-2">
        <DifficultyBadge difficulty={card.difficulty} />
        {card.kind === 'PRACTICE' && <Badge variant="muted">{kindLabel('PRACTICE')}</Badge>}
        <Badge variant="muted">{formatTimeLimit(card.timeLimitMinutes)}</Badge>
        <Badge variant="default">{status}</Badge>
      </div>
      {latest && (
        <p className="flex flex-wrap gap-x-3 text-muted-foreground">
          <span>{t('attempts.count', { count: card.attempts })}</span>
          <span>
            {t('sessionCard.latest', {
              outcome: latest.outcome === 'PASSED' ? t('common.passed') : t('common.failed'),
            })}
          </span>
          <span>{t('sessionCard.diagram', { score: latest.diagramScore })}</span>
          <span>{t('sessionCard.understanding', { score: latest.understandingScore })}</span>
        </p>
      )}
      <div className="flex gap-2">
        <Button size="sm" onClick={() => onStart(card)}>
          {latest ? t('catalog.startAgain') : t('sessionCard.open')}
        </Button>
        {latest && (
          <Button size="sm" variant="outline" onClick={() => onHistory(card)}>
            {t('common.history')}
          </Button>
        )}
      </div>
    </div>
  )
}

export { SessionExerciseCard }
