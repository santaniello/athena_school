import { Star } from 'lucide-react'
import type { LabCriterion } from '@/lib/lab'

interface LabRubricPanelProps {
  criteria: LabCriterion[]
}

// The rubric while the exam runs: each criterion with its weight and a marker
// on the mandatory ones. No score, level or percentage is shown.
function LabRubricPanel({ criteria }: LabRubricPanelProps) {
  return (
    <section className="flex flex-col gap-2 p-4">
      <h3 className="text-sm font-semibold text-foreground">Rubric</h3>
      <ul className="flex flex-col gap-2">
        {criteria.map((criterion) => (
          <li key={criterion.id} className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{criterion.name}</p>
              <p className="text-xs text-muted-foreground">{criterion.description}</p>
            </div>
            <span className="flex shrink-0 items-center gap-1 text-sm tabular-nums text-foreground">
              {criterion.mandatory && (
                <Star
                  className="size-3.5 fill-current text-primary"
                  aria-label="Mandatory"
                  role="img"
                />
              )}
              {criterion.weight}
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}

export { LabRubricPanel }
