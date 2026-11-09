import { DiagramEditor } from '@/components/lab/diagram-editor'

interface ResultDiagramProps {
  xml: string
}

function noop() {}

// The submitted diagram, in the same editor as the exam but locked. It needs
// the internet; without it the area says so and the rest of the page works.
export function ResultDiagram({ xml }: ResultDiagramProps) {
  return (
    <section aria-label="Submitted diagram" className="flex flex-col gap-3">
      <h3 className="text-lg font-semibold text-foreground">Submitted diagram</h3>
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
