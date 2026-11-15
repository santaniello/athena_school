import { Button } from '@/components/ui/button'
import { MAX_HINTS, type LabHint } from '@/lib/lab'
import { useTranslation } from 'react-i18next'

interface LabTutorPanelProps {
  hints: LabHint[]
  hintsLeft: number
  // A request is running: the button shows progress and cannot be pressed.
  asking: boolean
  // The last request failed and spent nothing.
  failed: boolean
  onAskHint: () => void
}

// Tutor Athena during the exam: empty until asked. The user can only request
// a hint; there is no message input. The hint behavior itself is owned by the
// caller. See specs/phases/phase-03-architecture-lab/08-lab-layout-sketch.md
// and 10-01-tutor-hints.md.
function LabTutorPanel({ hints, hintsLeft, asking, failed, onAskHint }: LabTutorPanelProps) {
  const { t } = useTranslation()
  return (
    <section className="flex flex-col gap-3 p-4">
      <h3 className="text-sm font-semibold text-foreground">{t('tutor.title')}</h3>
      <p className="text-sm text-muted-foreground">{t('tutor.intro')}</p>
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm text-foreground">
          {hintsLeft > 0
            ? t('tutor.hintsLeft', { left: hintsLeft, max: MAX_HINTS })
            : t('tutor.noHintsLeft')}
        </span>
        {hintsLeft > 0 && (
          <Button type="button" size="sm" variant="outline" disabled={asking} onClick={onAskHint}>
            {asking ? t('tutor.asking') : t('tutor.ask')}
          </Button>
        )}
      </div>
      {failed && (
        <p role="alert" className="text-sm text-destructive">
          {t('tutor.failed')}
        </p>
      )}
      {hints.length > 0 && (
        <div role="log" aria-label={t('tutor.hintsLabel')} className="flex flex-col gap-2">
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
