import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DefenseTranscript } from '@/components/lab/defense-transcript'
import { EvaluationTable } from '@/components/lab/evaluation-table'
import { LabStatementPanel } from '@/components/lab/lab-statement-panel'
import { OutcomeCard } from '@/components/lab/outcome-card'
import { ResultDiagram } from '@/components/lab/result-diagram'
import { ResultHeader } from '@/components/lab/result-header'
import { getAttemptResult, type AttemptResult } from '@/lib/exercise'

interface AttemptResultViewProps {
  attemptId: string
  backLabel: string
  onBack: () => void
}

// The result of a finished attempt: the last stage of the exam and what the
// history opens, so the user sees the same thing right after finishing and
// weeks later. It is read-only and shows what the backend returns. See
// specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.
export function AttemptResultView({ attemptId, backLabel, onBack }: AttemptResultViewProps) {
  const [result, setResult] = useState<AttemptResult | null>(null)
  const [failed, setFailed] = useState(false)
  // Bumping this reads the result again after a failure.
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let stale = false
    getAttemptResult(attemptId)
      .then((loaded) => !stale && setResult(loaded))
      .catch(() => !stale && setFailed(true))
    // Stryker disable next-line BooleanLiteral,BlockStatement: the flag only keeps a late answer from updating an unmounted view.
    return () => {
      stale = true
    }
  }, [attemptId, attempt])

  const backButton = (
    <Button type="button" variant="outline" onClick={onBack}>
      {backLabel}
    </Button>
  )

  if (failed) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-3 text-center">
        <p role="alert" className="text-sm text-destructive">
          Could not load the result.
        </p>
        <div className="flex gap-2">
          <Button
            type="button"
            onClick={() => {
              setFailed(false)
              setAttempt((current) => current + 1)
            }}
          >
            Try again
          </Button>
          {backButton}
        </div>
      </div>
    )
  }

  if (!result) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-3">
        <Loader2 className="size-5 animate-spin text-muted-foreground" aria-hidden="true" />
        <p role="status" className="text-sm text-muted-foreground">
          Loading the result...
        </p>
      </div>
    )
  }

  return (
    <div className="mx-auto flex h-full w-full max-w-3xl flex-col gap-6 overflow-y-auto p-6">
      <div>{backButton}</div>
      <ResultHeader result={result} />
      <section aria-label="Statement" className="rounded-md border border-border">
        <LabStatementPanel statement={result.challenge.statement} />
      </section>
      <OutcomeCard outcome={result.outcome} />
      <EvaluationTable
        title="Diagram evaluation"
        evaluation={result.diagram}
        notJudgedLabel="Not judged from the diagram"
      />
      <EvaluationTable
        title="Understanding evaluation"
        evaluation={result.understanding}
        notJudgedLabel="Not asked"
      />
      <DefenseTranscript turns={result.defense} />
      <ResultDiagram xml={result.submission} />
    </div>
  )
}
