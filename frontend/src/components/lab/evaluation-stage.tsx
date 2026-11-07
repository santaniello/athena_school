import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Evaluation } from '@/lib/exercise'

interface EvaluationStageProps {
  phase: 'evaluating' | 'evaluationFailed' | 'evaluated'
  // Why the evaluation failed; shown only in the failed state.
  message: string | null
  // The result; shown only once evaluated.
  evaluation: Evaluation | null
  onRetry: () => void
  onBack: () => void
}

// The exam after the diagram was submitted: progress while the evaluation
// runs, its failure with a manual retry, and the diagram result. Until the
// tutor's questions exist the stage ends with Back to catalog. See
// specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md.
export function EvaluationStage({
  phase,
  message,
  evaluation,
  onRetry,
  onBack,
}: EvaluationStageProps) {
  if (phase === 'evaluating') {
    return (
      <Centered>
        <Loader2 className="size-5 animate-spin text-muted-foreground" aria-hidden="true" />
        <p role="status" className="text-sm text-muted-foreground">
          Evaluating your diagram...
        </p>
      </Centered>
    )
  }

  if (phase === 'evaluationFailed' || !evaluation) {
    return (
      <Centered>
        <p role="alert" className="text-sm text-destructive">
          {message}
        </p>
        <div className="flex gap-2">
          <Button type="button" onClick={onRetry}>
            Try evaluating again
          </Button>
          <Button type="button" variant="outline" onClick={onBack}>
            Back to catalog
          </Button>
        </div>
      </Centered>
    )
  }

  return (
    <div className="mx-auto flex h-full w-full max-w-3xl flex-col gap-6 overflow-y-auto p-6">
      <header className="flex flex-col gap-2">
        <h1 className="text-lg font-semibold text-foreground">Diagram evaluated</h1>
        <p className="flex items-baseline gap-1">
          <span className="text-4xl font-semibold text-foreground">{evaluation.score}</span>
          <span className="text-sm text-muted-foreground">/ 100</span>
        </p>
        <p className="text-sm text-foreground">{evaluation.summary}</p>
      </header>
      <ul className="flex flex-col gap-3">
        {evaluation.results.map((result) => (
          <li
            key={result.key}
            aria-label={result.name}
            className="flex flex-col gap-1 rounded-md border border-border p-3"
          >
            <div className="flex items-baseline justify-between gap-2">
              <span className="text-sm font-medium text-foreground">{result.name}</span>
              <span className="text-xs text-muted-foreground">
                {result.insufficientEvidence
                  ? 'Not enough evidence in the diagram'
                  : `Level ${result.level} of 4`}
              </span>
            </div>
            <p className="text-sm text-foreground">{result.justification}</p>
            {result.evidence && <p className="text-xs text-muted-foreground">{result.evidence}</p>}
          </li>
        ))}
      </ul>
      <FeedbackList title="Strengths" items={evaluation.strengths} />
      <FeedbackList title="Gaps" items={evaluation.gaps} />
      <FeedbackList title="Recommendations" items={evaluation.recommendations} />
      <div>
        <Button type="button" onClick={onBack}>
          Back to catalog
        </Button>
      </div>
    </div>
  )
}

function FeedbackList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null
  return (
    <section className="flex flex-col gap-1">
      <h2 className="text-sm font-semibold text-foreground">{title}</h2>
      <ul className="list-disc pl-5 text-sm text-foreground">
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </section>
  )
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-center">
      {children}
    </div>
  )
}
