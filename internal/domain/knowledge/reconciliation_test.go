package knowledge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCandidate() Item {
	return Item{
		Topic:      "Distributed Systems",
		Concept:    "Idempotency key",
		Definition: "A unique value a client attaches to a request so retries produce the same effect exactly once.",
		Source:     SourceAthena,
		Status:     StatusDraft,
	}
}

func TestReconciliationProposal_Validate_acceptsEveryKnownActionWithItsRequiredTarget(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		action       string
		targetItemID string
	}{
		{action: ReconcileCreate, targetItemID: ""},
		{action: ReconcileUpdate, targetItemID: "item-1"},
		{action: ReconcileRelate, targetItemID: "item-1"},
		{action: ReconcileConflict, targetItemID: "item-1"},
		{action: ReconcileNoChange, targetItemID: "item-1"},
	}
	for _, test := range tests {
		t.Run(test.action, func(t *testing.T) {
			// Given a proposal for a known action with the target it requires
			proposal := ReconciliationProposal{
				ID: "proposal-1", Action: test.action, Status: ProposalPending,
				Candidate: validCandidate(), TargetItemID: test.targetItemID,
				Reason: "matches on concept and definition", CreatedAt: now,
			}

			// When validating it
			err := proposal.Validate()

			// Then it is accepted
			require.NoError(t, err)
		})
	}
}

func TestReconciliationProposal_Validate_rejectsInvalidFields(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	valid := ReconciliationProposal{
		ID: "proposal-1", Action: ReconcileUpdate, Status: ProposalPending,
		Candidate: validCandidate(), TargetItemID: "item-1",
		Reason: "matches on concept and definition", CreatedAt: now,
	}
	tests := []struct {
		name     string
		mutate   func(*ReconciliationProposal)
		expected error
	}{
		{name: "id", mutate: func(p *ReconciliationProposal) { p.ID = " " }, expected: ErrProposalIDRequired},
		{name: "unknown action", mutate: func(p *ReconciliationProposal) { p.Action = "merge" }, expected: ErrProposalActionInvalid},
		{name: "unknown status", mutate: func(p *ReconciliationProposal) { p.Status = "archived" }, expected: ErrProposalStatusInvalid},
		{name: "invalid candidate", mutate: func(p *ReconciliationProposal) { p.Candidate = Item{} }, expected: ErrProposalCandidateInvalid},
		{name: "missing target for update", mutate: func(p *ReconciliationProposal) { p.TargetItemID = " " }, expected: ErrProposalTargetRequired},
		{name: "missing reason", mutate: func(p *ReconciliationProposal) { p.Reason = " " }, expected: ErrProposalReasonRequired},
		{name: "missing created at", mutate: func(p *ReconciliationProposal) { p.CreatedAt = time.Time{} }, expected: ErrProposalCreatedAtRequired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)

			// When validating the invalid proposal
			err := candidate.Validate()

			// Then the matching domain error is returned
			assert.ErrorIs(t, err, test.expected)
		})
	}
}

func TestReconciliationProposal_Validate_rejectsATargetOnCreate(t *testing.T) {
	// Given a create proposal that carries a target item id — create never
	// mutates an existing item, so a target here is a contradiction, not a hint.
	proposal := ReconciliationProposal{
		ID: "proposal-1", Action: ReconcileCreate, Status: ProposalPending,
		Candidate: validCandidate(), TargetItemID: "item-1",
		Reason: "no existing match", CreatedAt: time.Now().UTC(),
	}

	// When validating it
	err := proposal.Validate()

	// Then it is rejected
	assert.ErrorIs(t, err, ErrProposalTargetForbidden)
}
