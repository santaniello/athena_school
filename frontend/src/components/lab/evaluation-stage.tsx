import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface EvaluationStageProps {
  phase: 'evaluating' | 'evaluationFailed'
  // Why the evaluation failed; shown only in the failed state.
  message: string | null
  onRetry: () => void
  backLabel: string
  onBack: () => void
}

// The exam after the diagram was submitted: progress while the evaluation
// runs, and its failure with a manual retry. A successful evaluation shows
// nothing: the tutor's questions start by themselves and the result appears
// only at the end. See
// specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md and
// 12-architecture-defense.md.
export function EvaluationStage({
  phase,
  message,
  backLabel,
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
          {backLabel}
        </Button>
      </div>
    </Centered>
  )
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-center">
      {children}
    </div>
  )
}
