import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { AlertCircle, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  generateProgressInsight,
  type ProgressInsightState,
  type ProgressStatement,
} from '@/lib/progress'

interface ProgressInsightCardProps {
  sessionId: string
  // The state the topic detail came with: the stored insight, if any.
  initial: ProgressInsightState
}

// Athena Insights of a topic: an analysis and recommendations the model wrote
// from the saved evidence. Opening the topic only shows what is stored; the
// model is called only when the user clicks Generate or Update. A failure
// keeps the previous insight and offers a retry, and never blocks the rest of
// the detail. Statements come from the model, so they are only ever rendered
// as text. See specs/phases/phase-05-progress/04-topic-insights.md.
export function ProgressInsightCard({ sessionId, initial }: ProgressInsightCardProps) {
  const { t, i18n } = useTranslation('progressTab')
  const headingId = useId()
  const [state, setState] = useState<ProgressInsightState>(initial)
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)

  const generate = async () => {
    setBusy(true)
    setFailed(false)
    try {
      setState(await generateProgressInsight(sessionId))
    } catch {
      setFailed(true)
    } finally {
      setBusy(false)
    }
  }

  const { insight } = state
  const buttonLabel = busy
    ? t('detail.insight.generating')
    : insight === null
      ? t('detail.insight.generate')
      : t('detail.insight.update')

  return (
    <section
      aria-labelledby={headingId}
      className="flex flex-col gap-3 rounded-2xl border border-primary/35 bg-muted px-[18px] py-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h4
          id={headingId}
          className="flex items-center gap-2 text-base font-extrabold text-primary"
        >
          <Sparkles className="size-5" aria-hidden="true" />
          {t('detail.insight.title')}
        </h4>
        {state.canGenerate && !failed && (
          <Button type="button" onClick={generate} disabled={busy}>
            {buttonLabel}
          </Button>
        )}
      </div>
      {state.stale && !failed && (
        <p className="rounded-lg bg-primary/10 px-3 py-2 text-sm text-primary">
          {t('detail.insight.stale')}
        </p>
      )}
      {failed && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p role="alert" className="flex items-center gap-2 text-sm text-destructive">
            <AlertCircle className="size-4 shrink-0" aria-hidden="true" />
            {insight === null ? t('detail.insight.failedFirst') : t('detail.insight.failed')}
          </p>
          <Button type="button" variant="outline" onClick={generate} disabled={busy}>
            {busy ? t('detail.insight.generating') : t('detail.insight.retry')}
          </Button>
        </div>
      )}
      {!state.canGenerate && insight === null && (
        <p className="text-sm text-muted-foreground">{t('detail.insight.blocked')}</p>
      )}
      {state.canGenerate && insight === null && !failed && (
        <p className="text-sm text-muted-foreground">{t('detail.insight.empty')}</p>
      )}
      {insight !== null && (
        <div className="flex flex-col gap-2.5">
          <p className="text-sm leading-relaxed text-foreground">
            {insight.analysis.map((sentence, index) => (
              <span key={index}>
                <Statement statement={sentence} />{' '}
              </span>
            ))}
          </p>
          <h5 className="text-sm font-extrabold text-foreground">
            {t('detail.insight.recommendations')}
          </h5>
          <ul className="flex flex-col gap-1.5">
            {insight.recommendations.map((recommendation, index) => (
              <li key={index} className="text-sm leading-normal text-foreground">
                • <Statement statement={recommendation} />
              </li>
            ))}
          </ul>
          <span className="text-xs text-muted-foreground">
            {t('detail.insight.meta', {
              date: new Date(insight.createdAt).toLocaleDateString(i18n.language),
              model: insight.model,
            })}
          </span>
        </div>
      )}
    </section>
  )
}

function Statement({ statement }: { statement: ProgressStatement }) {
  const { t } = useTranslation('progressTab')
  return (
    <>
      <span>{statement.text}</span>
      {statement.citations.length > 0 && (
        <span className="ml-1 text-xs text-muted-foreground">
          {t('detail.insight.basedOn', {
            sources: statement.citations.map((citation) => citation.label).join(', '),
          })}
        </span>
      )}
    </>
  )
}
