package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Reconciliation proposal actions. See
// specs/phases/phase-02-knowledge-engine/11-knowledge-reconciliation.md.
const (
	ReconcileCreate   = "create"
	ReconcileUpdate   = "update"
	ReconcileRelate   = "relate"
	ReconcileConflict = "conflict"
	ReconcileNoChange = "no_change"
)

// Reconciliation proposal statuses.
const (
	ProposalPending  = "pending"
	ProposalApplied  = "applied"
	ProposalRejected = "rejected"
	ProposalStale    = "stale"
)

// ReconciliationProposal persistence invariant errors, returned by
// ReconciliationProposal.Validate.
var (
	ErrProposalIDRequired        = errors.New("reconciliation proposal id is required")
	ErrProposalActionInvalid     = errors.New("reconciliation proposal action is invalid")
	ErrProposalStatusInvalid     = errors.New("reconciliation proposal status is invalid")
	ErrProposalCandidateInvalid  = errors.New("reconciliation proposal candidate is invalid")
	ErrProposalTargetRequired    = errors.New("reconciliation proposal target item id is required for this action")
	ErrProposalTargetForbidden   = errors.New("reconciliation proposal target item id must be empty for create")
	ErrProposalReasonRequired    = errors.New("reconciliation proposal reason is required")
	ErrProposalCreatedAtRequired = errors.New("reconciliation proposal created at is required")
)

// ItemChanges contains optional replacements for an existing Item's
// user-editable content fields — never its server-owned ID, Topic, Source,
// Status, or timestamps. A nil Definition, or a nil list field, means
// "leave this field unchanged"; ApplyProposal only overwrites what is set.
type ItemChanges struct {
	Definition      *string
	Properties      []string
	TradeOffs       []string
	RelatedConcepts []string
}

// ReconciliationProposal is the LLM's validated suggestion for how one
// extracted candidate relates to existing knowledge — the LLM proposes, the
// application validates, the user decides. See
// specs/phases/phase-02-knowledge-engine/11-knowledge-reconciliation.md.
type ReconciliationProposal struct {
	ID              string
	Action          string
	Status          string
	Candidate       Item
	TargetItemID    string
	TargetUpdatedAt time.Time
	Reason          string
	Changes         ItemChanges
	EvidenceIDs     []string
	CreatedAt       time.Time
}

// Validate checks the structural invariants every ReconciliationProposal
// must satisfy before it is acted on or persisted: a known action and
// status, a candidate that itself validates as an Item, a target present
// exactly when the action requires one, a non-blank reason, and a
// CreatedAt timestamp.
func (p ReconciliationProposal) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return ErrProposalIDRequired
	}
	if !knownReconcileAction(p.Action) {
		return ErrProposalActionInvalid
	}
	if !knownProposalStatus(p.Status) {
		return ErrProposalStatusInvalid
	}
	if err := p.Candidate.Validate(); err != nil {
		return ErrProposalCandidateInvalid
	}
	if p.Action == ReconcileCreate {
		if strings.TrimSpace(p.TargetItemID) != "" {
			return ErrProposalTargetForbidden
		}
	} else if strings.TrimSpace(p.TargetItemID) == "" {
		return ErrProposalTargetRequired
	}
	if strings.TrimSpace(p.Reason) == "" {
		return ErrProposalReasonRequired
	}
	if p.CreatedAt.IsZero() {
		return ErrProposalCreatedAtRequired
	}
	return nil
}

func knownReconcileAction(action string) bool {
	switch action {
	case ReconcileCreate, ReconcileUpdate, ReconcileRelate, ReconcileConflict, ReconcileNoChange:
		return true
	}
	return false
}

func knownProposalStatus(status string) bool {
	switch status {
	case ProposalPending, ProposalApplied, ProposalRejected, ProposalStale:
		return true
	}
	return false
}

// ReconciliationRepository persists ReconciliationProposal rows — the audit
// record kept for every reconciliation decision, applied immediately or
// saved for later — and their evidence links.
type ReconciliationRepository interface {
	// Save persists proposal's header row: candidate snapshot, action,
	// status, target, reason, changes and created_at. It never links
	// evidence — see LinkEvidence.
	Save(ctx context.Context, proposal ReconciliationProposal) error
	// LinkEvidence records that evidenceID (already persisted via
	// EvidenceRepository.GetOrCreate) supports proposalID.
	LinkEvidence(ctx context.Context, proposalID, evidenceID string) error
}
