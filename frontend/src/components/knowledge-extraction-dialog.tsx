import { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  acknowledgeReconciliationNoChange,
  applyReconciliationCreate,
  applyReconciliationRelate,
  applyReconciliationUpdate,
  CONFLICT_CREATE_SEPARATELY,
  CONFLICT_KEEP_EXISTING,
  CONFLICT_UPDATE_EXISTING,
  discardExtraction,
  RECONCILE_CONFLICT,
  RECONCILE_NO_CHANGE,
  RECONCILE_RELATE,
  RECONCILE_UPDATE,
  resolveReconciliationConflict,
  type KnowledgeItem,
} from '@/lib/knowledge'

// needsIndividualDecision reports whether item's classified action touches
// or links an existing item — update/relate/conflict — the three actions
// 11-knowledge-reconciliation.md requires the user to review individually
// rather than batch-approve. create and no_change stay in the batch zone:
// neither one ever mutates an existing item, so approving several of them
// at once carries none of that risk. A candidate whose classification is
// still missing (should not normally happen — see Service.classifyCandidates)
// is treated like create, the safe default.
function needsIndividualDecision(item: KnowledgeItem): boolean {
  const action = item.reconciliation?.action
  return action === RECONCILE_UPDATE || action === RECONCILE_RELATE || action === RECONCILE_CONFLICT
}

// targetMatch resolves item's classified target within its own duplicate
// shortlist, for its concept/status — the reconciliation suggestion itself
// only carries the target's id, never duplicating fields already present
// in `duplicates`.
function targetMatch(item: KnowledgeItem) {
  const targetId = item.reconciliation?.targetItemId
  if (!targetId) return undefined
  return (item.duplicates ?? []).find((match) => match.itemId === targetId)
}

interface DecisionState {
  pending: boolean
  done: boolean
  doneLabel: string
  error: string
}

const idleDecision: DecisionState = { pending: false, done: false, doneLabel: '', error: '' }

interface KnowledgeExtractionDialogProps {
  open: boolean
  batchId: string
  items: KnowledgeItem[]
  onClose: () => void
  // Fired after a decision that adds a new draft Knowledge Item — a create
  // saved as draft, a relate (always draft), or a conflict resolved as
  // "create separately" (always draft). Lets AppShell refresh the
  // sidebar/Review-tab badge without a reload. See
  // specs/phases/phase-02-knowledge-engine/07-knowledge-review.md.
  onKnowledgeChanged?: () => void
}

type BatchMode = 'draft' | 'approve'

export function KnowledgeExtractionDialog({
  open,
  batchId,
  items,
  onClose,
  onKnowledgeChanged,
}: KnowledgeExtractionDialogProps) {
  const decisionItems = items.filter(needsIndividualDecision)
  const batchItems = items.filter((item) => !needsIndividualDecision(item))

  const [decisions, setDecisions] = useState<Record<string, DecisionState>>({})
  const [batchSelected, setBatchSelected] = useState<Set<string>>(
    () => new Set(batchItems.map((item) => item.id)),
  )
  const [batchSaving, setBatchSaving] = useState<BatchMode | null>(null)
  const [batchError, setBatchError] = useState('')
  const [lastBatchMode, setLastBatchMode] = useState<BatchMode | null>(null)

  // batchId, not items, is the actual "this is a new batch" signal — items
  // is a fresh array reference most renders regardless, and this component
  // is remounted fresh per batch by its caller today, but resetting on
  // batchId keeps that a guarantee of the component itself rather than an
  // assumption about how any given caller renders it.
  const [resetForBatchId, setResetForBatchId] = useState(batchId)
  if (batchId !== resetForBatchId) {
    setResetForBatchId(batchId)
    setDecisions({})
    setBatchSelected(new Set(batchItems.map((item) => item.id)))
    setBatchSaving(null)
    setBatchError('')
    setLastBatchMode(null)
  }

  function decisionFor(id: string): DecisionState {
    return decisions[id] ?? idleDecision
  }

  function toggleBatchSelected(id: string) {
    setBatchSelected((previous) => {
      const next = new Set(previous)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleClose() {
    if (batchId) {
      try {
        await discardExtraction(batchId)
      } catch {
        // Best-effort: an unreachable backend during close should not block
        // the dialog from dismissing. The receipt simply outlives this
        // session's batch and stays orphaned until the app restarts.
      }
    }
    onClose()
  }

  async function runDecision(
    item: KnowledgeItem,
    addsDraft: boolean,
    action: () => Promise<unknown>,
  ) {
    setDecisions((previous) => ({ ...previous, [item.id]: { ...idleDecision, pending: true } }))
    try {
      await action()
      setDecisions((previous) => ({
        ...previous,
        [item.id]: { pending: false, done: true, doneLabel: 'Applied', error: '' },
      }))
      if (addsDraft) onKnowledgeChanged?.()
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : 'Something went wrong. Try again.'
      setDecisions((previous) => ({ ...previous, [item.id]: { ...idleDecision, error: message } }))
    }
  }

  function handleApplyUpdate(item: KnowledgeItem) {
    void runDecision(item, false, () => applyReconciliationUpdate(batchId, item.id, item))
  }

  function handleApplyRelate(item: KnowledgeItem) {
    void runDecision(item, true, () => applyReconciliationRelate(batchId, item.id, item))
  }

  function handleResolveConflict(item: KnowledgeItem, resolution: string) {
    const addsDraft = resolution === CONFLICT_CREATE_SEPARATELY
    void runDecision(item, addsDraft, () =>
      resolveReconciliationConflict(batchId, item.id, item, resolution),
    )
  }

  async function handleBatchSave(mode: BatchMode) {
    const pending = batchItems.filter(
      (item) => batchSelected.has(item.id) && !decisionFor(item.id).done,
    )
    if (pending.length === 0) return
    setBatchSaving(mode)
    setBatchError('')
    setLastBatchMode(mode)

    for (const item of pending) {
      setDecisions((previous) => ({ ...previous, [item.id]: { ...idleDecision, pending: true } }))
      try {
        const isNoChange = item.reconciliation?.action === RECONCILE_NO_CHANGE
        if (isNoChange) {
          await acknowledgeReconciliationNoChange(batchId, item.id, item)
        } else {
          await applyReconciliationCreate(
            batchId,
            item.id,
            item,
            mode === 'approve' ? 'approved' : 'draft',
          )
        }
        setDecisions((previous) => ({
          ...previous,
          [item.id]: {
            pending: false,
            done: true,
            doneLabel: isNoChange ? 'Acknowledged' : 'Saved',
            error: '',
          },
        }))
        if (!isNoChange && mode === 'draft') onKnowledgeChanged?.()
      } catch (caught) {
        const message = caught instanceof Error ? caught.message : 'Something went wrong.'
        setDecisions((previous) => ({
          ...previous,
          [item.id]: { ...idleDecision, error: message },
        }))
        setBatchError(message)
        setBatchSaving(null)
        return
      }
    }
    setBatchSaving(null)
  }

  function batchSaveLabel(mode: BatchMode, idleLabel: string) {
    if (lastBatchMode !== mode) return idleLabel
    if (batchSaving === mode) return 'Saving...'
    if (batchError) return 'Try again'
    return idleLabel
  }

  const pendingBatchCount = batchItems.filter(
    (item) => batchSelected.has(item.id) && !decisionFor(item.id).done,
  ).length
  const isSaving = batchSaving !== null

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && void handleClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New knowledge found</DialogTitle>
          <DialogDescription>
            {decisionItems.length > 0
              ? 'Review how each concept relates to what you already know, then choose which new concepts to save.'
              : 'Review the concepts and choose which ones to save.'}
          </DialogDescription>
        </DialogHeader>

        {items.length === 0 ? (
          <p className="py-4 text-sm text-muted-foreground">No new knowledge found</p>
        ) : (
          <div className="thin-scroll max-h-[60vh] space-y-4 overflow-y-auto">
            {decisionItems.length > 0 && (
              <div className="space-y-2">
                <p className="text-xs font-bold tracking-wide text-muted-foreground uppercase">
                  Needs your decision
                </p>
                {decisionItems.map((item) => (
                  <DecisionRow
                    key={item.id}
                    item={item}
                    state={decisionFor(item.id)}
                    onApplyUpdate={() => handleApplyUpdate(item)}
                    onApplyRelate={() => handleApplyRelate(item)}
                    onResolveConflict={(resolution) => handleResolveConflict(item, resolution)}
                  />
                ))}
              </div>
            )}

            {batchItems.length > 0 && (
              <div className="space-y-2">
                {decisionItems.length > 0 && (
                  <p className="text-xs font-bold tracking-wide text-muted-foreground uppercase">
                    Ready for batch save
                  </p>
                )}
                {batchItems.map((item) => {
                  const state = decisionFor(item.id)
                  const isNoChange = item.reconciliation?.action === RECONCILE_NO_CHANGE
                  const match = targetMatch(item)
                  return (
                    <label key={item.id} className="flex gap-3 rounded-lg border p-3">
                      <input
                        type="checkbox"
                        aria-label={`Select ${item.concept}`}
                        checked={batchSelected.has(item.id)}
                        disabled={state.done || isSaving}
                        onChange={() => toggleBatchSelected(item.id)}
                        className="mt-1 size-4 accent-primary"
                      />
                      <span className="min-w-0">
                        <span className="block font-medium text-foreground">{item.concept}</span>
                        <span className="mt-1 block text-sm text-muted-foreground">
                          {item.definition}
                        </span>
                        {isNoChange && match && (
                          <span className="mt-1 block text-xs text-muted-foreground">
                            Already captured by &ldquo;{match.concept}&rdquo; ({match.status}) —
                            nothing new to save.
                          </span>
                        )}
                        {item.reconciliationFailed && (
                          <span className="mt-1 block text-xs text-muted-foreground">
                            Comparison against existing knowledge was unavailable — showing as new.
                          </span>
                        )}
                        {state.error && (
                          <span className="mt-1 block text-xs text-destructive">{state.error}</span>
                        )}
                        {state.done && (
                          <span className="mt-1 block text-xs text-primary">{state.doneLabel}</span>
                        )}
                      </span>
                    </label>
                  )
                })}
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => void handleClose()} disabled={isSaving}>
            Dismiss
          </Button>
          {batchItems.length > 0 && (
            <>
              <Button
                variant="outline"
                onClick={() => void handleBatchSave('draft')}
                disabled={pendingBatchCount === 0 || isSaving}
              >
                {batchSaveLabel('draft', 'Save as drafts')}
              </Button>
              <Button
                onClick={() => void handleBatchSave('approve')}
                disabled={pendingBatchCount === 0 || isSaving}
              >
                {batchSaveLabel('approve', 'Save as knowledge')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

const actionLabel: Record<string, string> = {
  [RECONCILE_UPDATE]: 'Update',
  [RECONCILE_RELATE]: 'Relate',
  [RECONCILE_CONFLICT]: 'Conflict',
}

function DecisionRow({
  item,
  state,
  onApplyUpdate,
  onApplyRelate,
  onResolveConflict,
}: {
  item: KnowledgeItem
  state: DecisionState
  onApplyUpdate: () => void
  onApplyRelate: () => void
  onResolveConflict: (resolution: string) => void
}) {
  const action = item.reconciliation?.action ?? ''
  const match = targetMatch(item)
  const changes = item.reconciliation?.changes

  return (
    <div className="rounded-lg border p-3">
      <div className="flex items-center gap-2">
        <span className="font-medium text-foreground">{item.concept}</span>
        <span className="rounded-full border px-2 py-0.5 text-[0.65rem] font-bold tracking-wide text-muted-foreground uppercase">
          {actionLabel[action] ?? action}
        </span>
      </div>
      <p className="mt-1 text-sm text-muted-foreground">{item.definition}</p>
      {match && (
        <p className="mt-1 text-xs text-muted-foreground">
          Compares with &ldquo;{match.concept}&rdquo; ({match.status})
        </p>
      )}
      {item.reconciliation?.reason && (
        <p className="mt-1 text-xs text-muted-foreground italic">{item.reconciliation.reason}</p>
      )}
      {changes?.definition && (
        <p className="mt-1 text-xs text-foreground">New definition: {changes.definition}</p>
      )}

      {state.error && <p className="mt-2 text-xs text-destructive">{state.error}</p>}
      {state.done ? (
        <p className="mt-2 text-xs text-primary">{state.doneLabel}</p>
      ) : (
        <div className="mt-2 flex flex-wrap gap-2">
          {action === RECONCILE_UPDATE && (
            <Button size="sm" onClick={onApplyUpdate} disabled={state.pending}>
              {state.pending ? 'Applying...' : state.error ? 'Try again' : 'Apply update'}
            </Button>
          )}
          {action === RECONCILE_RELATE && (
            <Button size="sm" onClick={onApplyRelate} disabled={state.pending}>
              {state.pending ? 'Applying...' : state.error ? 'Try again' : 'Create & relate'}
            </Button>
          )}
          {action === RECONCILE_CONFLICT && (
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
        </div>
      )}
    </div>
  )
}
