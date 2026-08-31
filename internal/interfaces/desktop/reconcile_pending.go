package desktop

import (
	"errors"
	"time"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
)

// PendingReconciliationResult is the desktop-facing DTO for an
// applicationknowledge.PendingReconciliation — one pending proposal shown
// in Knowledge Review, already checked for staleness against its target's
// current state when the list was built.
type PendingReconciliationResult struct {
	ID            string              `json:"id"`
	Action        string              `json:"action"`
	Candidate     KnowledgeItemResult `json:"candidate"`
	TargetItemID  string              `json:"targetItemId"`
	TargetConcept string              `json:"targetConcept"`
	TargetStatus  string              `json:"targetStatus"`
	Reason        string              `json:"reason"`
	Changes       ItemChangesResult   `json:"changes"`
	Stale         bool                `json:"stale"`
	CreatedAt     string              `json:"createdAt"`
}

func toPendingReconciliationResult(p applicationknowledge.PendingReconciliation) PendingReconciliationResult {
	return PendingReconciliationResult{
		ID: p.Proposal.ID, Action: p.Proposal.Action, Candidate: toKnowledgeItemResult(p.Proposal.Candidate),
		TargetItemID: p.Proposal.TargetItemID, TargetConcept: p.TargetConcept, TargetStatus: p.TargetStatus,
		Reason: p.Proposal.Reason, Changes: toItemChangesResult(p.Proposal.Changes),
		Stale: p.Stale, CreatedAt: p.Proposal.CreatedAt.Format(time.RFC3339Nano),
	}
}

// ListPendingReconciliations lists every pending reconciliation proposal
// for the Knowledge Review screen, each already checked for staleness.
func (a *App) ListPendingReconciliations() ([]PendingReconciliationResult, error) {
	pending, err := a.knowledge.ListPendingReconciliations(a.ctx)
	if err != nil {
		return nil, err
	}
	results := make([]PendingReconciliationResult, len(pending))
	for index, p := range pending {
		results[index] = toPendingReconciliationResult(p)
	}
	return results, nil
}

// CountPendingReconciliations returns how many proposals currently sit
// pending — the Review badge count.
func (a *App) CountPendingReconciliations() (int, error) {
	return a.knowledge.CountPendingReconciliations(a.ctx)
}

// ApplyPendingReconciliationCreate persists proposalID's classified
// candidate as a brand-new Knowledge Item at status (draft or approved).
func (a *App) ApplyPendingReconciliationCreate(proposalID, status string) (KnowledgeItemResult, error) {
	if err := validateReconciliationProposalID(proposalID); err != nil {
		return KnowledgeItemResult{}, err
	}
	if err := validateKnowledgeStatus(status); err != nil {
		return KnowledgeItemResult{}, err
	}
	item, err := a.knowledge.ApplyPendingReconciliationCreate(a.ctx, proposalID, status)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("applying pending reconciliation create for proposal "+proposalID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// ApplyPendingReconciliationUpdate applies proposalID's classified changes
// to its target, preserving its identity and lifecycle.
func (a *App) ApplyPendingReconciliationUpdate(proposalID string) (KnowledgeItemResult, error) {
	if err := validateReconciliationProposalID(proposalID); err != nil {
		return KnowledgeItemResult{}, err
	}
	item, err := a.knowledge.ApplyPendingReconciliationUpdate(a.ctx, proposalID)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("applying pending reconciliation update for proposal "+proposalID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// ApplyPendingReconciliationRelate creates proposalID's candidate as a new
// draft Knowledge Item and links it to the classified target.
func (a *App) ApplyPendingReconciliationRelate(proposalID string) (KnowledgeItemResult, error) {
	if err := validateReconciliationProposalID(proposalID); err != nil {
		return KnowledgeItemResult{}, err
	}
	item, err := a.knowledge.ApplyPendingReconciliationRelate(a.ctx, proposalID)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("applying pending reconciliation relate for proposal "+proposalID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// ResolvePendingReconciliationConflict applies one of the three explicit
// conflict outcomes for proposalID: "keep_existing", "update_existing", or
// "create_separately".
func (a *App) ResolvePendingReconciliationConflict(proposalID, resolution string) (KnowledgeItemResult, error) {
	if err := validateReconciliationProposalID(proposalID); err != nil {
		return KnowledgeItemResult{}, err
	}
	if err := validateConflictResolution(resolution); err != nil {
		return KnowledgeItemResult{}, err
	}
	item, err := a.knowledge.ResolvePendingReconciliationConflict(a.ctx, proposalID, resolution)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("resolving pending reconciliation conflict for proposal "+proposalID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// AcknowledgePendingReconciliationNoChange marks proposalID's classified
// no_change proposal resolved without creating or changing any Item.
func (a *App) AcknowledgePendingReconciliationNoChange(proposalID string) error {
	if err := validateReconciliationProposalID(proposalID); err != nil {
		return err
	}
	return a.knowledge.AcknowledgePendingReconciliationNoChange(a.ctx, proposalID)
}

// RejectPendingReconciliationProposal marks proposalID rejected without
// creating or changing any Item.
func (a *App) RejectPendingReconciliationProposal(proposalID string) error {
	if err := validateReconciliationProposalID(proposalID); err != nil {
		return err
	}
	return a.knowledge.RejectPendingReconciliationProposal(a.ctx, proposalID)
}
