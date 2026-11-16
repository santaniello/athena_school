import { DiagramEditor } from '@/components/lab/diagram-editor'
import { useTranslation } from 'react-i18next'

interface ResultDiagramProps {
  xml: string
}

function noop() {}

// The submitted diagram, in the same editor as the exam but locked. It needs
// the internet; without it the area says so and the rest of the page works.
export function ResultDiagram({ xml }: ResultDiagramProps) {
  const { t } = useTranslation()
  return (
    <section aria-label={t('result.submittedDiagram')} className="flex flex-col gap-3">
      <h3 className="text-lg font-semibold text-foreground">{t('result.submittedDiagram')}</h3>
      <div className="relative h-[480px] overflow-hidden rounded-md border border-border">
        <DiagramEditor
          readOnly
          xml={xml}
          onReady={noop}
          onChange={noop}
          onConnectionLost={noop}
          onConnectionRestored={noop}
        />
      </div>
    </section>
  )
}
