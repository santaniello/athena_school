import type { LabStatement } from '@/lib/lab'
import { useTranslation } from 'react-i18next'

interface LabStatementPanelProps {
  statement: LabStatement
}

function StatementList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null
  return (
    <section className="flex flex-col gap-1.5">
      <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </section>
  )
}

// The exam statement: the scenario and the four lists. It never carries the
// rubric, a deliverables checklist or any hint. See
// specs/phases/phase-03-architecture-lab/08-lab-layout-sketch.md.
function LabStatementPanel({ statement }: LabStatementPanelProps) {
  const { t } = useTranslation()
  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-4">
      <section className="flex flex-col gap-1.5">
        <h3 className="text-sm font-semibold text-foreground">{t('statement.scenario')}</h3>
        <p className="text-sm whitespace-pre-line text-muted-foreground">{statement.scenario}</p>
      </section>
      <StatementList title={t('statement.functional')} items={statement.functionalRequirements} />
      <StatementList
        title={t('statement.nonFunctional')}
        items={statement.nonFunctionalRequirements}
      />
      <StatementList title={t('statement.constraints')} items={statement.constraints} />
      <StatementList title={t('statement.objectives')} items={statement.objectives} />
    </div>
  )
}

export { LabStatementPanel }
