import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DIFFICULTY_LABELS,
  STATUS_LABELS,
  type ChallengeDifficulty,
  type ChallengeStatus,
  type SessionExerciseCard as Card,
} from '@/lib/exercise'

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
  const difficulty = DIFFICULTY_LABELS[card.difficulty as ChallengeDifficulty] ?? card.difficulty
  const status = STATUS_LABELS[card.status as ChallengeStatus] ?? card.status
  const { latest } = card

  return (
    <div className="flex flex-col items-start gap-3 rounded-lg border bg-card p-3 text-sm">
      <p className="font-medium">{card.title}</p>
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="muted">{difficulty}</Badge>
        <Badge variant="muted">{card.timeLimitMinutes} min</Badge>
        <Badge variant="default">{status}</Badge>
      </div>
      {latest && (
        <p className="flex flex-wrap gap-x-3 text-muted-foreground">
          <span>
            {card.attempts} {card.attempts === 1 ? 'attempt' : 'attempts'}
          </span>
          <span>Latest: {latest.outcome === 'PASSED' ? 'Passed' : 'Failed'}</span>
          <span>Diagram {latest.diagramScore}</span>
          <span>Understanding {latest.understandingScore}</span>
        </p>
      )}
      <div className="flex gap-2">
        <Button size="sm" onClick={() => onStart(card)}>
          {latest ? 'Start again' : 'Open in Architecture Lab'}
        </Button>
        {latest && (
          <Button size="sm" variant="outline" onClick={() => onHistory(card)}>
            History
          </Button>
        )}
      </div>
    </div>
  )
}

export { SessionExerciseCard }
