import type { Evaluation } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface EvaluationTableProps {
  title: string
  evaluation: Evaluation
  // Shown instead of a level for a criterion that was not judged: the
  // diagram did not show it, or the tutor never reached it.
  notJudgedLabel: string
}

function FeedbackList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null
  return (
    <section className="flex flex-col gap-1">
      <h4 className="text-sm font-semibold text-foreground">{title}</h4>
      <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </section>
  )
}

// One evaluation, as stored: its score and summary, a row per criterion with
// weight, level, justification and evidence, and the strengths, gaps and
// recommendations. A criterion that was not judged says so and is never shown
// as level 0. See
// specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.
export function EvaluationTable({ title, evaluation, notJudgedLabel }: EvaluationTableProps) {
  const { t } = useTranslation()
  return (
    <section aria-label={title} className="flex flex-col gap-3">
      <header className="flex items-baseline justify-between gap-2">
        <h3 className="text-lg font-semibold text-foreground">{title}</h3>
        <p className="text-sm text-muted-foreground">
          {t('evaluationTable.score', { score: evaluation.score })}
        </p>
      </header>
      {evaluation.summary && (
        <p className="text-sm whitespace-pre-line text-muted-foreground">{evaluation.summary}</p>
      )}
      <ul className="divide-y divide-border rounded-md border border-border">
        {evaluation.results.map((result) => (
          <li key={result.key} className="flex flex-col gap-1 px-3 py-2">
            <p className="flex items-baseline justify-between gap-2 text-sm">
              <span className="font-medium text-foreground">
                {result.name}{' '}
                <span className="text-muted-foreground">
                  · {t('evaluationTable.weight', { weight: result.weight })}
                </span>
              </span>
              <span className="text-foreground">
                {result.insufficientEvidence
                  ? notJudgedLabel
                  : t('evaluationTable.level', { level: result.level })}
              </span>
            </p>
            {result.justification && (
              <p className="text-sm text-muted-foreground">{result.justification}</p>
            )}
            {result.evidence && (
              <p className="text-sm text-muted-foreground italic">
                {t('evaluationTable.evidence', { evidence: result.evidence })}
              </p>
            )}
          </li>
        ))}
      </ul>
      <FeedbackList title={t('evaluationTable.strengths')} items={evaluation.strengths} />
      <FeedbackList title={t('evaluationTable.gaps')} items={evaluation.gaps} />
      <FeedbackList
        title={t('evaluationTable.recommendations')}
        items={evaluation.recommendations}
      />
    </section>
  )
}
