import type { TFunction } from 'i18next'
import type { Outcome, OutcomeReason } from '@/lib/exercise'
import { useTranslation } from 'react-i18next'

interface OutcomeCardProps {
  outcome: Outcome
}

// Passed or Failed, both scores, the minimum used and, when failed, every
// reason. The outcome and the reasons come from the backend, never derived
// here. See specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.
export function OutcomeCard({ outcome }: OutcomeCardProps) {
  const { t } = useTranslation()
  const passed = outcome.result === 'PASSED'
  return (
    <section aria-label={t('outcome.label')} className="flex flex-col gap-4">
      <header className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold text-foreground">
          {passed ? t('common.passed') : t('common.failed')}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t('outcome.minimum', { score: outcome.passScore })}
        </p>
      </header>
      <div className="grid grid-cols-2 gap-3">
        <Score label={t('outcome.diagram')} value={outcome.diagramScore} />
        <Score label={t('outcome.understanding')} value={outcome.understandingScore} />
      </div>
      {outcome.reasons.length > 0 && (
        <ul className="list-disc pl-5 text-sm text-foreground">
          {outcome.reasons.map((reason, index) => (
            <li key={index}>{reasonText(reason, outcome.passScore, t)}</li>
          ))}
        </ul>
      )}
    </section>
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

function reasonText(reason: OutcomeReason, passScore: number, t: TFunction): string {
  switch (reason.kind) {
    case 'DIAGRAM_SCORE':
      return t('outcome.diagramBelow', { score: passScore })
    case 'UNDERSTANDING_SCORE':
      return t('outcome.understandingBelow', { score: passScore })
    case 'MANDATORY_CRITERION':
      return t(
        reason.evaluation === 'DIAGRAM' ? 'outcome.mandatoryDiagram' : 'outcome.mandatoryDefense',
        { name: reason.criterionName },
      )
  }
}
