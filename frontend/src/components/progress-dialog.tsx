import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { PROGRESS_NAMESPACE } from '@/i18n'
import { getStudySessionProgress, type StudyProgress, type StudyProgressConcept } from '@/lib/study'

interface ProgressDialogProps {
  open: boolean
  sessionId: string
  // True while a tutor reply is streaming. The state of a reply is stored once
  // it ends, so the progress is read again when this goes back to false.
  isStreaming?: boolean
  onClose: () => void
}

type ProgressState =
  { status: 'loading' } | { status: 'error' } | { status: 'ready'; progress: StudyProgress }

type BadgeVariant = 'beginner' | 'intermediate' | 'advanced' | 'muted'

// One color per group, like the difficulty badge: green is what the user
// showed, grey is only explained, amber is a gap and red is to review.
const GROUPS: readonly { key: keyof StudyProgress; label: string; variant: BadgeVariant }[] = [
  { key: 'demonstrated', label: 'groups.demonstrated', variant: 'beginner' },
  { key: 'explainedOnly', label: 'groups.explainedOnly', variant: 'muted' },
  { key: 'difficulties', label: 'groups.difficulties', variant: 'intermediate' },
  { key: 'needsReview', label: 'groups.needsReview', variant: 'advanced' },
]

const ASSESSMENTS: Record<string, { label: string; variant: BadgeVariant }> = {
  CORRECT: { label: 'assessment.correct', variant: 'beginner' },
  PARTIALLY_CORRECT: { label: 'assessment.partiallyCorrect', variant: 'intermediate' },
  INCORRECT: { label: 'assessment.incorrect', variant: 'advanced' },
}

function isEmpty(progress: StudyProgress): boolean {
  return GROUPS.every((group) => progress[group.key].length === 0)
}

// Concept names and descriptions are untrusted model output: they are only
// ever rendered as React text children, never as markup.
function ConceptEntry({ concept }: { concept: StudyProgressConcept }) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  return (
    <li className="space-y-2 rounded-lg border bg-card p-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <span className="font-medium break-words">{concept.name}</span>
        <span className="text-xs text-muted-foreground">
          {t('evidenceCount', { count: concept.evidenceCount })}
        </span>
      </div>
      {concept.evidence.length > 0 && (
        <ul className="space-y-1.5 border-t pt-2">
          {concept.evidence.length < concept.evidenceCount && (
            <li className="text-xs text-muted-foreground">
              {t('latestProofs', { count: concept.evidence.length })}
            </li>
          )}
          {concept.evidence.map((proof, index) => {
            const assessment = ASSESSMENTS[proof.assessment]
            return (
              <li key={`${proof.createdAt}-${index}`} className="space-y-0.5 text-xs">
                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge variant={assessment?.variant ?? 'muted'} className="px-2">
                    {assessment ? t(assessment.label) : proof.assessment}
                  </Badge>
                  {proof.assisted && (
                    <Badge variant="muted" className="px-2">
                      {t('assisted')}
                    </Badge>
                  )}
                </div>
                {proof.description && (
                  <p className="text-muted-foreground break-words">{proof.description}</p>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </li>
  )
}

// One tile per group with its count, empty groups included, so the shape of
// the session reads at a glance before the details.
function Overview({ progress }: { progress: StudyProgress }) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  return (
    <ul aria-label={t('overview')} className="grid grid-cols-2 gap-2 sm:grid-cols-4">
      {GROUPS.map((group) => (
        <li key={group.key} className="flex flex-col gap-1 rounded-lg border bg-card p-3">
          <Badge variant={group.variant} className="h-6 min-w-6 self-start px-2 text-xs">
            {progress[group.key].length}
          </Badge>
          <span className="text-xs text-muted-foreground">{t(group.label)}</span>
        </li>
      ))}
    </ul>
  )
}

// Mounted only while the dialog is open, so every opening reads the current
// progress and a closed dialog holds none.
function ProgressBody({ sessionId, isStreaming }: { sessionId: string; isStreaming: boolean }) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  const [state, setState] = useState<ProgressState>({ status: 'loading' })

  useEffect(() => {
    let cancelled = false
    getStudySessionProgress(sessionId)
      .then((progress) => {
        if (!cancelled) setState({ status: 'ready', progress })
      })
      .catch(() => {
        if (!cancelled) setState({ status: 'error' })
      })
    return () => {
      cancelled = true
    }
  }, [sessionId, isStreaming])

  if (state.status === 'loading') {
    return <p className="text-sm text-muted-foreground">{t('loading')}</p>
  }
  if (state.status === 'error') {
    return <p className="text-sm text-destructive">{t('loadFailed')}</p>
  }
  if (isEmpty(state.progress)) {
    return <p className="text-sm text-muted-foreground">{t('empty')}</p>
  }
  return (
    <div className="thin-scroll max-h-[70vh] space-y-5 overflow-y-auto pr-2 text-sm">
      <Overview progress={state.progress} />
      {GROUPS.filter((group) => state.progress[group.key].length > 0).map((group) => (
        <section key={group.key} aria-label={t(group.label)} className="space-y-2">
          <h3 className="flex items-center gap-2 font-semibold">
            {t(group.label)}
            <Badge variant={group.variant} className="px-2">
              {state.progress[group.key].length}
            </Badge>
          </h3>
          <ul className="space-y-2">
            {state.progress[group.key].map((concept) => (
              <ConceptEntry key={concept.name} concept={concept} />
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

// Read-only view of what the tutor observed in the session. It is derived from
// the stored state and proofs, with no model call, and says it is an estimate.
function ProgressDialog({ open, sessionId, isStreaming = false, onClose }: ProgressDialogProps) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-3xl" closeLabel={t('close')}>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>
        <ProgressBody sessionId={sessionId} isStreaming={isStreaming} />
      </DialogContent>
    </Dialog>
  )
}

export { ProgressDialog }
