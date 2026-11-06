import { Button } from '@/components/ui/button'
import { MAX_HINTS, type LabHint } from '@/lib/lab'

interface LabTutorPanelProps {
  hints: LabHint[]
  hintsLeft: number
  onAskHint: () => void
}

// Tutor Athena during the exam: empty until asked. The user can only request
// a hint; there is no message input. The hint behavior itself is owned by the
// caller. See specs/phases/phase-03-architecture-lab/08-lab-layout-sketch.md.
function LabTutorPanel({ hints, hintsLeft, onAskHint }: LabTutorPanelProps) {
  return (
    <section className="flex flex-col gap-3 p-4">
      <h3 className="text-sm font-semibold text-foreground">Tutor Athena</h3>
      <p className="text-sm text-muted-foreground">Ask for a hint if you get stuck.</p>
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm text-foreground">
          {hintsLeft > 0 ? `Hints left: ${hintsLeft} of ${MAX_HINTS}` : 'No hints left'}
        </span>
        {hintsLeft > 0 && (
          <Button type="button" size="sm" variant="outline" onClick={onAskHint}>
            Ask for a hint
          </Button>
        )}
      </div>
      {hints.length > 0 && (
        <div role="log" aria-label="Hints" className="flex flex-col gap-2">
          {hints.map((hint) => (
            <article
              key={hint.id}
              className="rounded-lg bg-muted px-3 py-2 text-sm text-foreground"
            >
              {hint.text}
            </article>
          ))}
        </div>
      )}
    </section>
  )
}

export { LabTutorPanel }
