import { useCallback, useEffect, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { AlertCircle, ChevronDown, ChevronUp, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  generateProgressFolderInsight,
  getProgressFolderInsight,
  type ProgressInsightState,
  type ProgressStatement,
} from '@/lib/progress'

interface ProgressFolderInsightCardProps {
  folderId: string
  // The sessions that are topics of the folder now: a citation of any other is
  // shown as plain text, since there is nothing to open.
  topicIds: string[]
  onOpenTopic: (sessionId: string) => void
}

type Loaded =
  { state: 'loading' } | { state: 'failed' } | { state: 'ready'; data: ProgressInsightState }

// Athena Insights of a folder: a consolidated analysis across its topics and
// what to study next, written by the model from the topic summaries. Showing
// the card only reads what is stored; the model is called only when the user
// clicks Generate or Update. It starts collapsed to the first sentence, and a
// failure keeps the previous insight and never blocks the topic list. Every
// statement comes from the model, so it is only ever rendered as text. See
// specs/phases/phase-05-progress/05-folder-insights.md.
export function ProgressFolderInsightCard({
  folderId,
  topicIds,
  onOpenTopic,
}: ProgressFolderInsightCardProps) {
  const { t, i18n } = useTranslation('progressTab')
  const headingId = useId()
  const [loaded, setLoaded] = useState<Loaded>({ state: 'loading' })
  const [reloadToken, setReloadToken] = useState(0)
  const [expanded, setExpanded] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let stale = false
    getProgressFolderInsight(folderId)
      .then((data) => {
        if (!stale) setLoaded({ state: 'ready', data })
      })
      .catch(() => {
        if (!stale) setLoaded({ state: 'failed' })
      })
    return () => {
      stale = true
    }
  }, [folderId, reloadToken])

  const generate = useCallback(async () => {
    setBusy(true)
    setFailed(false)
    try {
      const fresh = await generateProgressFolderInsight(folderId)
      // A folder that lost its evidence answers with no insight: the stored one
      // is still there, so keep showing it and only drop the button.
      setLoaded((current) =>
        fresh.insight === null && current.state === 'ready' && current.data.insight !== null
          ? { state: 'ready', data: { ...current.data, canGenerate: fresh.canGenerate } }
          : { state: 'ready', data: fresh },
      )
    } catch {
      setFailed(true)
    } finally {
      setBusy(false)
    }
  }, [folderId])

  const heading = (
    <h4 id={headingId} className="flex items-center gap-2 text-base font-extrabold text-primary">
      <Sparkles className="size-5" aria-hidden="true" />
      {t('detail.insight.title')}
    </h4>
  )
  const shell = 'flex flex-col gap-3 rounded-2xl border border-primary/35 bg-muted px-[18px] py-4'

  if (loaded.state === 'loading') {
    return (
      <section aria-labelledby={headingId} className={shell}>
        {heading}
        <p className="text-sm text-muted-foreground">{t('folderInsight.loading')}</p>
      </section>
    )
  }
  if (loaded.state === 'failed') {
    return (
      <section aria-labelledby={headingId} className={shell}>
        {heading}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p role="alert" className="flex items-center gap-2 text-sm text-destructive">
            <AlertCircle className="size-4 shrink-0" aria-hidden="true" />
            {t('folderInsight.loadFailed')}
          </p>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setLoaded({ state: 'loading' })
              setReloadToken((token) => token + 1)
            }}
          >
            {t('detail.insight.retry')}
          </Button>
        </div>
      </section>
    )
  }

  const { insight, stale, canGenerate } = loaded.data
  const buttonLabel = busy
    ? t('detail.insight.generating')
    : insight === null
      ? t('detail.insight.generate')
      : t('detail.insight.update')

  return (
    <section aria-labelledby={headingId} className={shell}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        {heading}
        {canGenerate && !failed && (
          <Button type="button" onClick={generate} disabled={busy}>
            {buttonLabel}
          </Button>
        )}
      </div>
      {stale && !failed && (
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
      {!canGenerate && insight === null && (
        <p className="text-sm text-muted-foreground">{t('folderInsight.blocked')}</p>
      )}
      {canGenerate && insight === null && !failed && (
        <p className="text-sm text-muted-foreground">{t('folderInsight.empty')}</p>
      )}
      {insight !== null && (
        <div className="flex flex-col gap-2.5">
          <p className="text-sm leading-relaxed text-foreground">
            {(expanded ? insight.analysis : insight.analysis.slice(0, 1)).map((sentence, index) => (
              <span key={index}>
                <Statement
                  statement={sentence}
                  topicIds={topicIds}
                  onOpenTopic={onOpenTopic}
                />{' '}
              </span>
            ))}
          </p>
          {expanded && (
            <>
              <h5 className="text-sm font-extrabold text-foreground">
                {t('detail.insight.recommendations')}
              </h5>
              <ul className="flex flex-col gap-1.5">
                {insight.recommendations.map((recommendation, index) => (
                  <li key={index} className="text-sm leading-normal text-foreground">
                    •{' '}
                    <Statement
                      statement={recommendation}
                      topicIds={topicIds}
                      onOpenTopic={onOpenTopic}
                    />
                  </li>
                ))}
              </ul>
              <span className="text-xs text-muted-foreground">
                {t('detail.insight.meta', {
                  date: new Date(insight.createdAt).toLocaleDateString(i18n.language),
                  model: insight.model,
                })}
              </span>
            </>
          )}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="self-start"
            aria-expanded={expanded}
            onClick={() => setExpanded((open) => !open)}
          >
            {expanded ? (
              <ChevronUp className="size-4" aria-hidden="true" />
            ) : (
              <ChevronDown className="size-4" aria-hidden="true" />
            )}
            {expanded ? t('folderInsight.showLess') : t('folderInsight.showMore')}
          </Button>
        </div>
      )}
    </section>
  )
}

function Statement({
  statement,
  topicIds,
  onOpenTopic,
}: {
  statement: ProgressStatement
  topicIds: string[]
  onOpenTopic: (sessionId: string) => void
}) {
  const { t } = useTranslation('progressTab')
  return (
    <>
      <span>{statement.text}</span>
      {statement.citations.length > 0 && (
        <span className="ml-1 text-xs text-muted-foreground">
          {t('detail.insight.basedOnPrefix')}{' '}
          {statement.citations.map((citation, index) => (
            <span key={citation.id}>
              {index > 0 && ', '}
              {topicIds.includes(citation.id) ? (
                <button
                  type="button"
                  className="cursor-pointer text-primary underline-offset-2 hover:underline"
                  onClick={() => onOpenTopic(citation.id)}
                >
                  {citation.label}
                </button>
              ) : (
                citation.label
              )}
            </span>
          ))}
        </span>
      )}
    </>
  )
}
