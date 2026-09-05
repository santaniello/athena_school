package knowledge

import (
	"context"
	"errors"
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// PendingReconciliation is one pending proposal shown in Knowledge Review,
// enriched with an eager staleness check against its target's current
// state — computed once, when the list loads, so the review screen never
// lets the user click into a surprise failure. TargetConcept/TargetStatus
// are populated only when the proposal has a target and it is not stale.
type PendingReconciliation struct {
	Proposal      domainknowledge.ReconciliationProposal
	Stale         bool
	TargetConcept string
	TargetStatus  string
}

// ListPendingReconciliations lists every pending proposal, oldest first,
// checking each one's target freshness as it loads — a target that was
// edited or removed since classification marks that entry Stale without
// preventing the rest of the list from loading.
func (s *Service) ListPendingReconciliations(ctx context.Context) ([]PendingReconciliation, error) {
	proposals, err := s.reconciliations.ListByStatus(ctx, domainknowledge.ProposalPending)
	if err != nil {
		return nil, err
	}
	results := make([]PendingReconciliation, len(proposals))
	for index, proposal := range proposals {
		result := PendingReconciliation{Proposal: proposal}
		if proposal.TargetItemID != "" {
			target, err := s.checkReconciliationTargetFresh(ctx, proposal.TargetItemID, proposal.TargetUpdatedAt)
			if errors.Is(err, ErrReconciliationTargetStale) {
				result.Stale = true
			} else {
				if err != nil {
					return nil, err
				}
				result.TargetConcept = target.Concept
				result.TargetStatus = target.Status
			}
		}
		results[index] = result
	}
	return results, nil
}

// CountPendingReconciliations returns how many proposals currently sit
// pending — the Review badge count.
func (s *Service) CountPendingReconciliations(ctx context.Context) (int, error) {
	return s.reconciliations.CountByStatus(ctx, domainknowledge.ProposalPending)
}

// ApplyPendingReconciliationCreate persists proposalID's classified
// candidate as a brand-new Knowledge Item at status, using exactly the
// candidate content and evidence already persisted when it was saved for
// review — never the original study session, which may no longer exist.
func (s *Service) ApplyPendingReconciliationCreate(ctx context.Context, proposalID, status string) (domainknowledge.Item, error) {
	return s.applyPendingReconciliationMutation(ctx, proposalID, "",
		func(ctx context.Context, proposal domainknowledge.ReconciliationProposal, _ domainknowledge.Item) (domainknowledge.Item, error) {
			return s.createReconciledItem(ctx, proposal.Candidate, status)
		},
	)
}

// ApplyPendingReconciliationUpdate applies the classified ItemChanges to
// proposalID's target, preserving its identity, lifecycle and creation time.
func (s *Service) ApplyPendingReconciliationUpdate(ctx context.Context, proposalID string) (domainknowledge.Item, error) {
	return s.applyPendingReconciliationMutation(ctx, proposalID, "",
		func(ctx context.Context, proposal domainknowledge.ReconciliationProposal, target domainknowledge.Item) (domainknowledge.Item, error) {
			return s.updateReconciledItem(ctx, target, proposal.Changes)
		},
	)
}

// ApplyPendingReconciliationRelate creates proposalID's candidate as a new
// draft Knowledge Item and links it to the classified target via a
// canonical `related` relation.
func (s *Service) ApplyPendingReconciliationRelate(ctx context.Context, proposalID string) (domainknowledge.Item, error) {
	return s.applyPendingReconciliationMutation(ctx, proposalID, "",
		func(ctx context.Context, proposal domainknowledge.ReconciliationProposal, target domainknowledge.Item) (domainknowledge.Item, error) {
			item, err := s.createReconciledItem(ctx, proposal.Candidate, domainknowledge.StatusDraft)
			if err != nil {
				return domainknowledge.Item{}, err
			}
			relation := domainknowledge.CanonicalRelation(item.ID, target.ID, domainknowledge.RelationRelated, time.Now().UTC())
			if err := s.relations.Save(ctx, relation); err != nil {
				return domainknowledge.Item{}, err
			}
			return item, nil
		},
	)
}

// ResolvePendingReconciliationConflict applies one of the three explicit
// conflict outcomes to proposalID — see ResolveReconciliationConflict for
// the same rule applied to an immediate decision: every outcome keeps the
// proposal's Action as `conflict`, recording the chosen resolution in its
// Reason instead.
func (s *Service) ResolvePendingReconciliationConflict(ctx context.Context, proposalID, resolution string) (domainknowledge.Item, error) {
	switch resolution {
	case ConflictKeepExisting:
		err := s.resolvePendingReconciliationNoMutation(ctx, proposalID, "resolved: kept existing item")
		return domainknowledge.Item{}, err
	case ConflictUpdateExisting:
		return s.applyPendingReconciliationMutation(ctx, proposalID, "resolved: updated existing item",
			func(ctx context.Context, proposal domainknowledge.ReconciliationProposal, target domainknowledge.Item) (domainknowledge.Item, error) {
				return s.updateReconciledItem(ctx, target, proposal.Changes)
			},
		)
	case ConflictCreateSeparately:
		return s.applyPendingReconciliationMutation(ctx, proposalID, "resolved: created separately",
			func(ctx context.Context, proposal domainknowledge.ReconciliationProposal, _ domainknowledge.Item) (domainknowledge.Item, error) {
				return s.createReconciledItem(ctx, proposal.Candidate, domainknowledge.StatusDraft)
			},
		)
	default:
		return domainknowledge.Item{}, ErrReconciliationResolutionInvalid
	}
}

// AcknowledgePendingReconciliationNoChange marks proposalID's classified
// no_change proposal applied without creating or changing any Item.
func (s *Service) AcknowledgePendingReconciliationNoChange(ctx context.Context, proposalID string) error {
	return s.resolvePendingReconciliationNoMutation(ctx, proposalID, "")
}

// RejectPendingReconciliationProposal marks proposalID rejected without
// creating or changing any Item. Unlike every other pending action, this
// never checks target freshness — rejecting applies nothing, so a target
// that changed since classification does not need to block it.
func (s *Service) RejectPendingReconciliationProposal(ctx context.Context, proposalID string) error {
	proposal, err := s.reconciliations.GetByID(ctx, proposalID)
	if err != nil {
		return err
	}
	if proposal.Status != domainknowledge.ProposalPending {
		return ErrReconciliationProposalNotPending
	}
	return s.reconciliations.UpdateStatus(ctx, proposalID, domainknowledge.ProposalRejected, proposal.Reason, time.Now().UTC())
}

// applyPendingReconciliationMutation is ListPendingReconciliations' apply-
// side counterpart for every action that creates or changes a Knowledge
// Item. It reloads proposalID from persisted storage — never a transient
// receipt, since a pending proposal must survive an app restart — refuses
// to act on anything but a still-pending proposal, enforces the same
// staleness check applyReconciliationMutation does when the proposal has a
// target, runs mutate, links the proposal's already-materialized evidence
// to the resulting item (no session reload, no quote revalidation — that
// already happened once, at "Save for review" time), and marks the
// proposal applied. Indexing runs post-commit, same as everywhere else.
func (s *Service) applyPendingReconciliationMutation(
	ctx context.Context, proposalID, resolutionNote string,
	mutate func(ctx context.Context, proposal domainknowledge.ReconciliationProposal, target domainknowledge.Item) (domainknowledge.Item, error),
) (domainknowledge.Item, error) {
	if err := s.index.BeginMutation(); err != nil {
		return domainknowledge.Item{}, err
	}
	defer s.index.EndMutation()

	proposal, err := s.reconciliations.GetByID(ctx, proposalID)
	if err != nil {
		return domainknowledge.Item{}, err
	}
	if proposal.Status != domainknowledge.ProposalPending {
		return domainknowledge.Item{}, ErrReconciliationProposalNotPending
	}

	var resultItem domainknowledge.Item
	txErr := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var target domainknowledge.Item
		if proposal.TargetItemID != "" {
			var err error
			target, err = s.checkReconciliationTargetFresh(ctx, proposal.TargetItemID, proposal.TargetUpdatedAt)
			if err != nil {
				return err
			}
		}
		item, mutateErr := mutate(ctx, proposal, target)
		if mutateErr != nil {
			return mutateErr
		}
		resultItem = item
		if err := s.reconciliations.UpdateStatus(ctx, proposal.ID, domainknowledge.ProposalApplied, reasonWithResolution(proposal.Reason, resolutionNote), time.Now().UTC()); err != nil {
			return err
		}
		for _, evidenceID := range proposal.EvidenceIDs {
			if err := s.evidence.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: item.ID, EvidenceID: evidenceID}); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return domainknowledge.Item{}, txErr
	}

	if err := s.indexKnowledgeItem(ctx, resultItem); err != nil {
		return resultItem, err
	}
	return resultItem, nil
}

// resolvePendingReconciliationNoMutation is applyPendingReconciliationMutation's
// counterpart for a decision that never creates or changes an Item:
// acknowledging no_change, or keeping the existing item on a conflict.
func (s *Service) resolvePendingReconciliationNoMutation(ctx context.Context, proposalID, resolutionNote string) error {
	proposal, err := s.reconciliations.GetByID(ctx, proposalID)
	if err != nil {
		return err
	}
	if proposal.Status != domainknowledge.ProposalPending {
		return ErrReconciliationProposalNotPending
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if proposal.TargetItemID != "" {
			if _, err := s.checkReconciliationTargetFresh(ctx, proposal.TargetItemID, proposal.TargetUpdatedAt); err != nil {
				return err
			}
		}
		return s.reconciliations.UpdateStatus(ctx, proposal.ID, domainknowledge.ProposalApplied, reasonWithResolution(proposal.Reason, resolutionNote), time.Now().UTC())
	})
}

func reasonWithResolution(reason, resolutionNote string) string {
	if resolutionNote == "" {
		return reason
	}
	return reason + "; " + resolutionNote
}
