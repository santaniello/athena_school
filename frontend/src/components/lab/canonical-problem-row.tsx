import { DifficultyBadge } from '@/components/difficulty-badge'
import { Button } from '@/components/ui/button'
import { difficultyStateLabel, generatedCount, type CanonicalProblem } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface CanonicalProblemRowProps {
  problem: CanonicalProblem
  // Start was clicked; the caller asks for the difficulty.
  onStart: (problem: CanonicalProblem) => void
  // History was clicked; the caller opens the finished attempts of the problem.
  onHistory: (problem: CanonicalProblem) => void
}

// One row of the Interview tab. Like every catalog row it shows only what does
// not give the exam away: the problem's name and description, how many
// difficulties are generated and the attempts, never a scenario or a rubric.
// See specs/phases/phase-07-improve-arch-lab/03-canonical-interviews.md.
function CanonicalProblemRow({ problem, onStart, onHistory }: CanonicalProblemRowProps) {
  const { t } = useTranslation()
  const attempts =
    problem.attempts === 0
      ? t('attempts.notStarted')
      : t('attempts.count', { count: problem.attempts })

  return (
    <li className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3">
      <div className="min-w-0 flex-1 basis-48">
        <p className="truncate text-sm font-semibold text-foreground">{problem.name}</p>
        <p className="text-sm text-muted-foreground">{problem.summary}</p>
        <p className="text-sm text-muted-foreground">
          {t('canonical.generated', {
            generated: generatedCount(problem),
            total: problem.difficulties.length,
          })}{' '}
          · {attempts}
        </p>
      </div>
      <div className="flex flex-wrap items-center justify-end gap-3">
        <div className="flex items-center gap-1.5">
          {problem.difficulties.map((state) => (
            // The level of each challenge of the problem; one not generated yet is dimmed.
            <span
              key={state.difficulty}
              title={difficultyStateLabel(state)}
              className={state.generated ? undefined : 'opacity-40'}
            >
              <DifficultyBadge difficulty={state.difficulty} />
            </span>
          ))}
        </div>
        {problem.hasHistory && (
          <Button type="button" variant="outline" size="sm" onClick={() => onHistory(problem)}>
            {t('common.history')}
          </Button>
        )}
        <Button type="button" variant="outline" size="sm" onClick={() => onStart(problem)}>
          {t('catalog.start')}
        </Button>
      </div>
    </li>
  )
}

export { CanonicalProblemRow }
