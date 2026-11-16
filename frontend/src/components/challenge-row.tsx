import { DifficultyBadge } from '@/components/difficulty-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatTimeLimit, kindLabel, statusLabel, type CatalogItem } from '@/lib/exercise'
import { cn } from '@/lib/utils'
import { useTranslation } from 'react-i18next'

interface ChallengeRowProps {
  item: CatalogItem
  // True for the challenge that was just generated.
  highlighted: boolean
  // Start or Start again was clicked; the caller opens the rules dialog.
  onStart: (item: CatalogItem) => void
  // History was clicked; the caller opens the list of finished attempts.
  onHistory: (item: CatalogItem) => void
}

// One catalog row. It shows only what does not give the exam away: never the
// scenario, requirements or rubric. Start opens the exam rules. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md and
// 10-diagram-workspace.md.
function ChallengeRow({ item, highlighted, onStart, onHistory }: ChallengeRowProps) {
  const { t } = useTranslation()
  const status = statusLabel(item.status)
  const attempts =
    item.attempts === 0 ? t('attempts.notStarted') : t('attempts.count', { count: item.attempts })

  return (
    <li
      aria-current={highlighted ? 'true' : undefined}
      className={cn(
        'flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3',
        highlighted && 'bg-primary/10 ring-1 ring-primary/40',
      )}
    >
      <div className="min-w-0 flex-1 basis-48">
        <p className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <span className="truncate">{item.title}</span>
          {item.kind === 'PRACTICE' && <Badge variant="muted">{kindLabel('PRACTICE')}</Badge>}
        </p>
        <p className="text-sm text-muted-foreground">
          {item.topic} · {formatTimeLimit(item.timeLimitMinutes)} · {attempts}
        </p>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-3">
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
            {t('common.history')}
          </Button>
        )}
        <Button type="button" variant="outline" size="sm" onClick={() => onStart(item)}>
          {item.attempts === 0 ? t('catalog.start') : t('catalog.startAgain')}
        </Button>
      </div>
    </li>
  )
}

export { ChallengeRow }
