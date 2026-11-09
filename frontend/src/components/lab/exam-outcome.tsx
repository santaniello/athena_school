import { Button } from '@/components/ui/button'
import type { Outcome, OutcomeReason } from '@/lib/exercise'

interface ExamOutcomeProps {
  outcome: Outcome
  onBack: () => void
}

// The end of the exam: Passed or Failed, both scores and, when failed, every
// reason. See specs/phases/phase-03-architecture-lab/12-architecture-defense.md.
export function ExamOutcome({ outcome, onBack }: ExamOutcomeProps) {
  const passed = outcome.result === 'PASSED'
  return (
    <div className="mx-auto flex h-full w-full max-w-xl flex-col gap-6 overflow-y-auto p-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold text-foreground">{passed ? 'Passed' : 'Failed'}</h1>
        <p className="text-sm text-muted-foreground">Minimum to pass: {outcome.passScore}</p>
      </header>
      <div className="grid grid-cols-2 gap-3">
        <Score label="Diagram" value={outcome.diagramScore} />
        <Score label="Understanding" value={outcome.understandingScore} />
      </div>
      {outcome.reasons.length > 0 && (
        <ul className="list-disc pl-5 text-sm text-foreground">
          {outcome.reasons.map((reason, index) => (
            <li key={index}>{reasonText(reason, outcome.passScore)}</li>
          ))}
        </ul>
      )}
      <div>
        <Button type="button" onClick={onBack}>
          Back to catalog
        </Button>
      </div>
    </div>
  )
}

function Score({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex flex-col gap-1 rounded-md border border-border p-3">
      <span className="text-sm text-muted-foreground">{label}</span>
      <p className="flex items-baseline gap-1">
        <span className="text-3xl font-semibold text-foreground">{value}</span>
        <span className="text-sm text-muted-foreground">/ 100</span>
      </p>
    </div>
  )
}

function reasonText(reason: OutcomeReason, passScore: number): string {
  switch (reason.kind) {
    case 'DIAGRAM_SCORE':
      return `The diagram score is below the minimum of ${passScore}.`
    case 'UNDERSTANDING_SCORE':
      return `The understanding score is below the minimum of ${passScore}.`
    case 'MANDATORY_CRITERION': {
      const where = reason.evaluation === 'DIAGRAM' ? 'diagram' : 'defense'
      return `Mandatory criterion not met in the ${where}: ${reason.criterionName}.`
    }
  }
}
