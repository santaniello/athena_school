import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PROGRESS_NAMESPACE } from '@/i18n'
import { getStudySessionProgress, type StudyProgress, type StudyProgressConcept } from '@/lib/study'

interface ProgressDialogProps {
  open: boolean
  sessionId: string
  onClose: () => void
}

type ProgressState =
  { status: 'loading' } | { status: 'error' } | { status: 'ready'; progress: StudyProgress }

const GROUPS = [
  { key: 'demonstrated', label: 'groups.demonstrated' },
  { key: 'explainedOnly', label: 'groups.explainedOnly' },
  { key: 'difficulties', label: 'groups.difficulties' },
  { key: 'needsReview', label: 'groups.needsReview' },
] as const

const ASSESSMENT_LABELS: Record<string, string> = {
  CORRECT: 'assessment.correct',
  PARTIALLY_CORRECT: 'assessment.partiallyCorrect',
  INCORRECT: 'assessment.incorrect',
}

function isEmpty(progress: StudyProgress): boolean {
  return GROUPS.every((group) => progress[group.key].length === 0)
}

// Concept names and descriptions are untrusted model output: they are only
// ever rendered as React text children, never as markup.
function ConceptEntry({ concept }: { concept: StudyProgressConcept }) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  return (
    <li className="space-y-1">
      <div className="flex flex-wrap items-baseline gap-x-2">
        <span className="font-medium">{concept.name}</span>
        <span className="text-xs text-muted-foreground">
          {t('evidenceCount', { count: concept.evidenceCount })}
        </span>
      </div>
      {concept.evidence.length > 0 && (
        <ul className="space-y-0.5 pl-4 text-xs text-muted-foreground">
          {concept.evidence.map((proof, index) => (
            <li key={`${proof.createdAt}-${index}`} className="flex flex-wrap gap-x-2">
              <span className="font-medium">
                {ASSESSMENT_LABELS[proof.assessment]
                  ? t(ASSESSMENT_LABELS[proof.assessment])
                  : proof.assessment}
              </span>
              {proof.assisted && <span>{t('assisted')}</span>}
              {proof.description && <span>{proof.description}</span>}
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

// Mounted only while the dialog is open, so every opening reads the current
// progress and a closed dialog holds none.
function ProgressBody({ sessionId }: { sessionId: string }) {
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
  }, [sessionId])

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
    <div className="thin-scroll max-h-[60vh] space-y-4 overflow-y-auto pr-2 text-sm">
      {GROUPS.filter((group) => state.progress[group.key].length > 0).map((group) => (
        <section key={group.key} aria-label={t(group.label)} className="space-y-2">
          <h3 className="font-semibold">{t(group.label)}</h3>
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
function ProgressDialog({ open, sessionId, onClose }: ProgressDialogProps) {
  const { t } = useTranslation(PROGRESS_NAMESPACE)
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>
        <ProgressBody sessionId={sessionId} />
      </DialogContent>
    </Dialog>
  )
}

export { ProgressDialog }
