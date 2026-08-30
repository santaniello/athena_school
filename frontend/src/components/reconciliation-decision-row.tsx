import { Button } from '@/components/ui/button'
import {
  CONFLICT_CREATE_SEPARATELY,
  CONFLICT_KEEP_EXISTING,
  CONFLICT_UPDATE_EXISTING,
  RECONCILE_CONFLICT,
  RECONCILE_RELATE,
  RECONCILE_UPDATE,
} from '@/lib/knowledge'

// DecisionState tracks one row's own async apply/resolve call, independent
// of any batch action elsewhere on the screen — see
// specs/phases/phase-02-knowledge-engine/11-knowledge-reconciliation.md.
export interface DecisionState {
  pending: boolean
  done: boolean
  doneLabel: string
  error: string
}

export const idleDecision: DecisionState = { pending: false, done: false, doneLabel: '', error: '' }

const actionLabel: Record<string, string> = {
  [RECONCILE_UPDATE]: 'Update',
  [RECONCILE_RELATE]: 'Relate',
  [RECONCILE_CONFLICT]: 'Conflict',
}

export interface ReconciliationDecisionRowProps {
  concept: string
  definition: string
  action: string
  targetConcept?: string
  targetStatus?: string
  reason?: string
  newDefinition?: string
  // stale is only ever true for a proposal loaded from Knowledge Review —
  // the extraction dialog's own decisions are always fresh, since nothing
  // has had time to change since classification ran moments earlier.
  stale?: boolean
  state: DecisionState
  onApplyUpdate?: () => void
  onApplyRelate?: () => void
  onResolveConflict?: (resolution: string) => void
  // onReject is only offered from Knowledge Review — the extraction
  // dialog's "do nothing" is simply not clicking anything, then Dismiss.
  onReject?: () => void
  // onSaveForReview is only offered from the extraction dialog — a
  // proposal already sitting in Knowledge Review has nowhere further to
  // be saved for.
  onSaveForReview?: () => void
}

// ReconciliationDecisionRow renders one update/relate/conflict candidate
// with its own immediate, independently-resolving actions — shared between
// the extraction dialog (a freshly classified candidate) and Knowledge
// Review (a proposal saved for later), so both present the identical
// action vocabulary and visual treatment.
export function ReconciliationDecisionRow({
  concept,
  definition,
  action,
  targetConcept,
  targetStatus,
  reason,
  newDefinition,
  stale,
  state,
  onApplyUpdate,
  onApplyRelate,
  onResolveConflict,
  onReject,
  onSaveForReview,
}: ReconciliationDecisionRowProps) {
  return (
    <div className="rounded-lg border p-3">
      <div className="flex items-center gap-2">
        <span className="font-medium text-foreground">{concept}</span>
        <span className="rounded-full border px-2 py-0.5 text-[0.65rem] font-bold tracking-wide text-muted-foreground uppercase">
          {actionLabel[action] ?? action}
        </span>
        {stale && (
          <span className="rounded-full border border-destructive/40 px-2 py-0.5 text-[0.65rem] font-bold tracking-wide text-destructive uppercase">
            Stale
          </span>
        )}
      </div>
      <p className="mt-1 text-sm text-muted-foreground">{definition}</p>
      {targetConcept && (
        <p className="mt-1 text-xs text-muted-foreground">
          Compares with &ldquo;{targetConcept}&rdquo; ({targetStatus})
        </p>
      )}
      {reason && <p className="mt-1 text-xs text-muted-foreground italic">{reason}</p>}
      {newDefinition && (
        <p className="mt-1 text-xs text-foreground">New definition: {newDefinition}</p>
      )}

      {state.error && <p className="mt-2 text-xs text-destructive">{state.error}</p>}
      {state.done ? (
        <p className="mt-2 text-xs text-primary">{state.doneLabel}</p>
      ) : stale ? (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <span className="text-xs text-muted-foreground">
            This changed since it was saved for review.
          </span>
          {onReject && (
            <Button size="sm" variant="outline" onClick={onReject} disabled={state.pending}>
              Reject
            </Button>
          )}
        </div>
      ) : (
        <div className="mt-2 flex flex-wrap gap-2">
          {action === RECONCILE_UPDATE && onApplyUpdate && (
            <Button size="sm" onClick={onApplyUpdate} disabled={state.pending}>
              {state.pending ? 'Applying...' : state.error ? 'Try again' : 'Apply update'}
            </Button>
          )}
          {action === RECONCILE_RELATE && onApplyRelate && (
            <Button size="sm" onClick={onApplyRelate} disabled={state.pending}>
              {state.pending ? 'Applying...' : state.error ? 'Try again' : 'Create & relate'}
            </Button>
          )}
          {action === RECONCILE_CONFLICT && onResolveConflict && (
            <>
              <Button
                size="sm"
                variant="outline"
                onClick={() => onResolveConflict(CONFLICT_KEEP_EXISTING)}
                disabled={state.pending}
              >
                Keep existing
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => onResolveConflict(CONFLICT_UPDATE_EXISTING)}
                disabled={state.pending}
              >
                Update existing
              </Button>
              <Button
                size="sm"
                onClick={() => onResolveConflict(CONFLICT_CREATE_SEPARATELY)}
                disabled={state.pending}
              >
                Create separately
              </Button>
            </>
          )}
          {onSaveForReview && (
            <Button size="sm" variant="outline" onClick={onSaveForReview} disabled={state.pending}>
              Save for review
            </Button>
          )}
          {onReject && (
            <Button size="sm" variant="outline" onClick={onReject} disabled={state.pending}>
              Reject
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
