package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

func newTestReconciliationRepository(t *testing.T) (*ReconciliationRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO knowledge_items
		(id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at)
		VALUES ('item-target', 'Distributed Systems', 'Eventual consistency', 'Converges eventually.', '[]', '[]', '[]', 'athena', 'approved', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return NewReconciliationRepository(db), db
}

func testReconciliationCandidate() domainknowledge.Item {
	return domainknowledge.Item{
		Topic:      "Distributed Systems",
		Concept:    "Eventual consistency",
		Definition: "Reads eventually reflect the latest write once updates stop.",
		Source:     domainknowledge.SourceAthena,
		Status:     domainknowledge.StatusDraft,
	}
}

func TestReconciliationRepository_SavePersistsAnAppliedProposalWithAResolvedTimestamp(t *testing.T) {
	// Given an applied update proposal targeting an existing item
	repository, db := newTestReconciliationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	newDefinition := "Reads eventually reflect the latest write once updates stop, with no read-your-writes guarantee."
	proposal := domainknowledge.ReconciliationProposal{
		ID: "proposal-1", Action: domainknowledge.ReconcileUpdate, Status: domainknowledge.ProposalApplied,
		Candidate: testReconciliationCandidate(), TargetItemID: "item-target", TargetUpdatedAt: createdAt,
		Reason: "same concept, extends the definition", Changes: domainknowledge.ItemChanges{Definition: &newDefinition},
		CreatedAt: createdAt,
	}

	// When saving it
	err := repository.Save(ctx, proposal)

	// Then the row is persisted with a non-null resolved_at, since it did
	// not stay pending
	require.NoError(t, err)
	var action, status, targetItemID string
	var resolvedAt sql.NullTime
	require.NoError(t, db.QueryRow(
		`SELECT action, status, target_item_id, resolved_at FROM knowledge_reconciliation_proposals WHERE id = ?`, proposal.ID,
	).Scan(&action, &status, &targetItemID, &resolvedAt))
	assert.Equal(t, domainknowledge.ReconcileUpdate, action)
	assert.Equal(t, domainknowledge.ProposalApplied, status)
	assert.Equal(t, "item-target", targetItemID)
	assert.True(t, resolvedAt.Valid)
}

func TestReconciliationRepository_SavePersistsAPendingProposalWithNoResolvedTimestampAndNoTarget(t *testing.T) {
	// Given a pending create proposal — no existing match, so no target
	repository, db := newTestReconciliationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	proposal := domainknowledge.ReconciliationProposal{
		ID: "proposal-2", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: createdAt,
	}

	// When saving it for review
	err := repository.Save(ctx, proposal)

	// Then the row is persisted with no target and no resolved_at
	require.NoError(t, err)
	var targetItemID sql.NullString
	var targetUpdatedAt, resolvedAt sql.NullTime
	require.NoError(t, db.QueryRow(
		`SELECT target_item_id, target_updated_at, resolved_at FROM knowledge_reconciliation_proposals WHERE id = ?`, proposal.ID,
	).Scan(&targetItemID, &targetUpdatedAt, &resolvedAt))
	assert.False(t, targetItemID.Valid)
	assert.False(t, targetUpdatedAt.Valid)
	assert.False(t, resolvedAt.Valid)
}

func TestReconciliationRepository_LinkEvidenceAttachesAnEvidenceSnapshotToAProposal(t *testing.T) {
	// Given a saved proposal and a persisted Evidence snapshot
	repository, db := newTestReconciliationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	proposal := domainknowledge.ReconciliationProposal{
		ID: "proposal-3", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalApplied,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: createdAt,
	}
	require.NoError(t, repository.Save(ctx, proposal))
	evidenceRepo := NewEvidenceRepository(db)
	evidence, err := evidenceRepo.GetOrCreate(ctx, domainknowledge.Evidence{
		ID: "evidence-1", OriginType: domainknowledge.OriginSessionMessage, OriginID: "message-1",
		SourceLabel: "Distributed Systems", Excerpt: "literal quote", CreatedAt: createdAt,
	})
	require.NoError(t, err)

	// When linking the evidence to the proposal
	linkErr := repository.LinkEvidence(ctx, proposal.ID, evidence.ID)

	// Then the link is persisted
	require.NoError(t, linkErr)
	var count int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM knowledge_reconciliation_evidence WHERE proposal_id = ? AND evidence_id = ?`,
		proposal.ID, evidence.ID,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestReconciliationRepository_GetByIDReturnsTheFullProposalWithItsLinkedEvidence(t *testing.T) {
	// Given a pending proposal with two linked evidence snapshots
	repository, db := newTestReconciliationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	newDefinition := "Reads eventually reflect the latest write once updates stop, with no read-your-writes guarantee."
	proposal := domainknowledge.ReconciliationProposal{
		ID: "proposal-1", Action: domainknowledge.ReconcileUpdate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), TargetItemID: "item-target", TargetUpdatedAt: createdAt,
		Reason: "same concept, extends the definition", Changes: domainknowledge.ItemChanges{Definition: &newDefinition},
		CreatedAt: createdAt,
	}
	require.NoError(t, repository.Save(ctx, proposal))
	evidenceRepo := NewEvidenceRepository(db)
	evidenceA, err := evidenceRepo.GetOrCreate(ctx, domainknowledge.Evidence{
		ID: "evidence-a", OriginType: domainknowledge.OriginSessionMessage, OriginID: "message-1",
		SourceLabel: "Distributed Systems", Excerpt: "quote a", CreatedAt: createdAt,
	})
	require.NoError(t, err)
	evidenceB, err := evidenceRepo.GetOrCreate(ctx, domainknowledge.Evidence{
		ID: "evidence-b", OriginType: domainknowledge.OriginSessionMessage, OriginID: "message-2",
		SourceLabel: "Distributed Systems", Excerpt: "quote b", CreatedAt: createdAt,
	})
	require.NoError(t, err)
	require.NoError(t, repository.LinkEvidence(ctx, proposal.ID, evidenceA.ID))
	require.NoError(t, repository.LinkEvidence(ctx, proposal.ID, evidenceB.ID))

	// When reloading it by ID
	loaded, err := repository.GetByID(ctx, proposal.ID)

	// Then every field round-trips, including its evidence links
	require.NoError(t, err)
	assert.Equal(t, proposal.ID, loaded.ID)
	assert.Equal(t, proposal.Action, loaded.Action)
	assert.Equal(t, proposal.Status, loaded.Status)
	assert.Equal(t, proposal.Candidate, loaded.Candidate)
	assert.Equal(t, proposal.TargetItemID, loaded.TargetItemID)
	assert.True(t, proposal.TargetUpdatedAt.Equal(loaded.TargetUpdatedAt))
	assert.Equal(t, proposal.Reason, loaded.Reason)
	require.NotNil(t, loaded.Changes.Definition)
	assert.Equal(t, newDefinition, *loaded.Changes.Definition)
	assert.ElementsMatch(t, []string{"evidence-a", "evidence-b"}, loaded.EvidenceIDs)
}

func TestReconciliationRepository_GetByIDReturnsNotFoundForAMissingProposal(t *testing.T) {
	// Given no proposal with this id
	repository, _ := newTestReconciliationRepository(t)

	// When loading it
	_, err := repository.GetByID(context.Background(), "missing")

	// Then it reports not found
	assert.ErrorIs(t, err, domainknowledge.ErrProposalNotFound)
}

func TestReconciliationRepository_ListByStatusReturnsOnlyMatchingProposalsOldestFirst(t *testing.T) {
	// Given two pending proposals and one already applied
	repository, _ := newTestReconciliationRepository(t)
	ctx := context.Background()
	older := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-newer", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: newer,
	}))
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-older", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: older,
	}))
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-applied", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalApplied,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: older,
	}))

	// When listing pending proposals
	pending, err := repository.ListByStatus(ctx, domainknowledge.ProposalPending)

	// Then only the two pending ones are returned, oldest first
	require.NoError(t, err)
	require.Len(t, pending, 2)
	assert.Equal(t, "proposal-older", pending[0].ID)
	assert.Equal(t, "proposal-newer", pending[1].ID)
}

func TestReconciliationRepository_UpdateStatusTransitionsAndStampsResolvedAt(t *testing.T) {
	// Given a pending proposal
	repository, db := newTestReconciliationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-1", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: createdAt,
	}))
	resolvedAt := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)

	// When rejecting it, with a resolution note appended to its reason
	err := repository.UpdateStatus(ctx, "proposal-1", domainknowledge.ProposalRejected, "no existing match; rejected by user", resolvedAt)

	// Then the status, reason and resolved_at are all updated
	require.NoError(t, err)
	var status, reason string
	var storedResolvedAt time.Time
	require.NoError(t, db.QueryRow(
		`SELECT status, reason, resolved_at FROM knowledge_reconciliation_proposals WHERE id = ?`, "proposal-1",
	).Scan(&status, &reason, &storedResolvedAt))
	assert.Equal(t, domainknowledge.ProposalRejected, status)
	assert.Equal(t, "no existing match; rejected by user", reason)
	assert.True(t, resolvedAt.Equal(storedResolvedAt))
}

func TestReconciliationRepository_UpdateStatusReturnsNotFoundForAMissingProposal(t *testing.T) {
	// Given no proposal with this id
	repository, _ := newTestReconciliationRepository(t)

	// When updating its status
	err := repository.UpdateStatus(context.Background(), "missing", domainknowledge.ProposalRejected, "rejected", time.Now().UTC())

	// Then it reports not found
	assert.ErrorIs(t, err, domainknowledge.ErrProposalNotFound)
}

func TestReconciliationRepository_CountByStatusCountsOnlyMatchingProposals(t *testing.T) {
	// Given two pending proposals and one applied
	repository, _ := newTestReconciliationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-1", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: createdAt,
	}))
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-2", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalPending,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: createdAt,
	}))
	require.NoError(t, repository.Save(ctx, domainknowledge.ReconciliationProposal{
		ID: "proposal-3", Action: domainknowledge.ReconcileCreate, Status: domainknowledge.ProposalApplied,
		Candidate: testReconciliationCandidate(), Reason: "no existing match", CreatedAt: createdAt,
	}))

	// When counting pending proposals
	count, err := repository.CountByStatus(ctx, domainknowledge.ProposalPending)

	// Then only the pending ones are counted
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
