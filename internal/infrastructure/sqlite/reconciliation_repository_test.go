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
