import { useEffect, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  idleDecision,
  ReconciliationDecisionRow,
  type DecisionState,
} from '@/components/reconciliation-decision-row'
import {
  applyPendingReconciliationRelate,
  applyPendingReconciliationUpdate,
  listPendingReconciliations,
  rejectPendingReconciliationProposal,
  resolvePendingReconciliationConflict,
  type PendingReconciliation,
} from '@/lib/knowledge'

interface PendingReconciliationSectionProps {
  // Fired after any resolution (applied, rejected, or an already-stale
  // proposal is rejected) — every one of them changes how many proposals
  // are pending, so AppShell can refresh the combined Review badge
  // (draftCount + pendingProposalCount) without a reload. See
  // specs/phases/phase-02-knowledge-engine/07-knowledge-review.md.
  onKnowledgeChanged?: () => void
}

// PendingReconciliationSection lists proposals saved for later review — see
// specs/phases/phase-02-knowledge-engine/11-knowledge-reconciliation.md.
// Only update/relate/conflict candidates ever reach "Save for review" from
// the extraction dialog, so every row here renders through the same
// ReconciliationDecisionRow the dialog's own decision zone uses. Resolving
// a proposal (applying, or rejecting) removes it from this list — unlike
// the extraction dialog's one-shot batch review, this is a live queue, not
// a record of one session's decisions.
export function PendingReconciliationSection({
  onKnowledgeChanged,
}: PendingReconciliationSectionProps) {
  const [proposals, setProposals] = useState<PendingReconciliation[]>([])
  const [loadError, setLoadError] = useState('')
  const [decisions, setDecisions] = useState<Record<string, DecisionState>>({})

  // On mount only — proposals are removed from local state as they resolve
  // (see runDecision), so nothing else here needs a full reload.
  useEffect(() => {
    let ignore = false
    listPendingReconciliations()
      .then((loaded) => {
        if (!ignore) setProposals(loaded)
      })
      .catch(() => {
        if (!ignore) setLoadError('Failed to load pending proposals.')
      })
    return () => {
      ignore = true
    }
  }, [])

  function decisionFor(id: string): DecisionState {
    return decisions[id] ?? idleDecision
  }

  async function runDecision(id: string, action: () => Promise<unknown>) {
    setDecisions((previous) => ({ ...previous, [id]: { ...idleDecision, pending: true } }))
    try {
      await action()
      setProposals((previous) => previous.filter((proposal) => proposal.id !== id))
      onKnowledgeChanged?.()
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : 'Something went wrong. Try again.'
      setDecisions((previous) => ({ ...previous, [id]: { ...idleDecision, error: message } }))
    }
  }

  if (!loadError && proposals.length === 0) return null

  return (
    <div className="space-y-2">
      <p className="text-xs font-bold tracking-wide text-muted-foreground uppercase">
        Pending reconciliation
      </p>
      {loadError && (
        <Alert variant="destructive">
          <AlertDescription>{loadError}</AlertDescription>
        </Alert>
      )}
      {proposals.map((proposal) => (
        <ReconciliationDecisionRow
          key={proposal.id}
          concept={proposal.candidate.concept}
          definition={proposal.candidate.definition}
          action={proposal.action}
          targetConcept={proposal.targetConcept || undefined}
          targetStatus={proposal.targetStatus || undefined}
          reason={proposal.reason}
          newDefinition={proposal.changes.definition}
          stale={proposal.stale}
          state={decisionFor(proposal.id)}
          onApplyUpdate={() =>
            void runDecision(proposal.id, () => applyPendingReconciliationUpdate(proposal.id))
          }
          onApplyRelate={() =>
            void runDecision(proposal.id, () => applyPendingReconciliationRelate(proposal.id))
          }
          onResolveConflict={(resolution) =>
            void runDecision(proposal.id, () =>
              resolvePendingReconciliationConflict(proposal.id, resolution),
            )
          }
          onReject={() =>
            void runDecision(proposal.id, () => rejectPendingReconciliationProposal(proposal.id))
          }
        />
      ))}
    </div>
  )
}
