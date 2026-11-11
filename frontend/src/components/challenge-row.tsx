import { DifficultyBadge } from '@/components/difficulty-badge'
import { Button } from '@/components/ui/button'
import { STATUS_LABELS, type CatalogItem, type ChallengeStatus } from '@/lib/exercise'
import { cn } from '@/lib/utils'

interface ChallengeRowProps {
  item: CatalogItem
  // True for the challenge that was just generated.
  highlighted: boolean
  // Start or Start again was clicked; the caller opens the rules dialog.
  onStart: (item: CatalogItem) => void
  // History was clicked; the caller opens the list of finished attempts.
  onHistory: (item: CatalogItem) => void
}

function attemptsText(attempts: number): string {
  if (attempts === 0) return 'not started'
  return `${attempts} ${attempts === 1 ? 'attempt' : 'attempts'}`
}

// One catalog row. It shows only what does not give the exam away: never the
// scenario, requirements or rubric. Start opens the exam rules. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md and
// 10-diagram-workspace.md.
function ChallengeRow({ item, highlighted, onStart, onHistory }: ChallengeRowProps) {
  const status = STATUS_LABELS[item.status as ChallengeStatus] ?? item.status

  return (
    <li
      aria-current={highlighted ? 'true' : undefined}
      className={cn(
        'flex items-center justify-between gap-4 px-4 py-3',
        highlighted && 'bg-primary/10 ring-1 ring-primary/40',
      )}
    >
      <div className="min-w-0">
        <p className="truncate text-sm font-semibold text-foreground">{item.title}</p>
        <p className="text-sm text-muted-foreground">
          {item.topic} · {item.timeLimitMinutes} min · {attemptsText(item.attempts)}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <div className="flex items-center gap-3">
          {item.status !== 'NOT_STARTED' && (
            <span
              className={cn(
                'text-sm text-muted-foreground',
                item.status === 'NEEDS_IMPROVEMENT' && 'font-semibold',
              )}
            >
              {status}
            </span>
          )}
          <DifficultyBadge difficulty={item.difficulty} />
        </div>
        {item.attempts > 0 && (
          <Button type="button" variant="outline" size="sm" onClick={() => onHistory(item)}>
            History
          </Button>
        )}
        <Button type="button" variant="outline" size="sm" onClick={() => onStart(item)}>
          {item.attempts === 0 ? 'Start' : 'Start again'}
        </Button>
      </div>
    </li>
  )
}

export { ChallengeRow }
