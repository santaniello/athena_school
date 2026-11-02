import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import {
  DIFFICULTY_LABELS,
  STATUS_LABELS,
  type CatalogItem,
  type ChallengeDifficulty,
  type ChallengeStatus,
} from '@/lib/exercise'
import { cn } from '@/lib/utils'

interface ChallengeRowProps {
  item: CatalogItem
  // True for the challenge that was just generated.
  highlighted: boolean
}

function attemptsText(attempts: number): string {
  if (attempts === 0) return 'not started'
  return `${attempts} ${attempts === 1 ? 'attempt' : 'attempts'}`
}

// One catalog row. It shows only what does not give the exam away: never the
// scenario, requirements or rubric. Start is disabled until the diagram
// editor exists. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.
function ChallengeRow({ item, highlighted }: ChallengeRowProps) {
  const difficulty = DIFFICULTY_LABELS[item.difficulty as ChallengeDifficulty] ?? item.difficulty
  const status = STATUS_LABELS[item.status as ChallengeStatus] ?? item.status
  const progress = item.bestDiagramScore > 0 ? `Best ${item.bestDiagramScore} · ${status}` : status

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
        <div className="flex flex-col items-end gap-1">
          <Badge variant="muted">{difficulty}</Badge>
          {item.status !== 'NOT_STARTED' && (
            <span className="text-sm text-muted-foreground">{progress}</span>
          )}
        </div>
        <Tooltip>
          <TooltipTrigger asChild>
            {/* A disabled button swallows pointer events, so the tooltip hangs on its wrapper. */}
            <span data-testid="start-tooltip-trigger" tabIndex={0}>
              <Button type="button" variant="outline" size="sm" disabled>
                {item.attempts === 0 ? 'Start' : 'Start again'}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>Available with the diagram editor</TooltipContent>
        </Tooltip>
      </div>
    </li>
  )
}

export { ChallengeRow }
