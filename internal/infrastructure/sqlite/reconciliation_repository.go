package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// ReconciliationRepository is the SQLite-backed
// knowledge.ReconciliationRepository.
type ReconciliationRepository struct {
	db *sql.DB
}

// NewReconciliationRepository creates a ReconciliationRepository backed by db.
func NewReconciliationRepository(db *sql.DB) *ReconciliationRepository {
	return &ReconciliationRepository{db: db}
}

// Save persists proposal's header row. A target-less proposal (create)
// stores NULL for target_item_id/target_updated_at rather than an empty
// string/zero time. resolved_at is set to CreatedAt for every non-pending
// status — this increment always creates and resolves a proposal within
// the same call, so there is no real gap between the two — and left NULL
// while the proposal stays pending, for the review flow that later acts on
// it to fill in.
func (r *ReconciliationRepository) Save(ctx context.Context, proposal knowledge.ReconciliationProposal) error {
	candidateSnapshot, err := json.Marshal(proposal.Candidate)
	if err != nil {
		return fmt.Errorf("sqlite: encoding reconciliation candidate snapshot: %w", err)
	}
	changes, err := json.Marshal(proposal.Changes)
	if err != nil {
		return fmt.Errorf("sqlite: encoding reconciliation changes: %w", err)
	}

	var targetItemID sql.NullString
	var targetUpdatedAt sql.NullTime
	if proposal.TargetItemID != "" {
		targetItemID = sql.NullString{String: proposal.TargetItemID, Valid: true}
		targetUpdatedAt = sql.NullTime{Time: proposal.TargetUpdatedAt, Valid: true}
	}
	var resolvedAt sql.NullTime
	if proposal.Status != knowledge.ProposalPending {
		resolvedAt = sql.NullTime{Time: proposal.CreatedAt, Valid: true}
	}

	_, err = execer(ctx, r.db).ExecContext(ctx, `INSERT INTO knowledge_reconciliation_proposals
		(id, action, status, candidate_snapshot, target_item_id, target_updated_at, reason, changes, created_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proposal.ID, proposal.Action, proposal.Status, string(candidateSnapshot),
		targetItemID, targetUpdatedAt, proposal.Reason, string(changes), proposal.CreatedAt, resolvedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving reconciliation proposal: %w", err)
	}
	return nil
}

// LinkEvidence records that evidenceID supports proposalID.
func (r *ReconciliationRepository) LinkEvidence(ctx context.Context, proposalID, evidenceID string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO knowledge_reconciliation_evidence (proposal_id, evidence_id) VALUES (?, ?)`,
		proposalID, evidenceID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: linking reconciliation evidence: %w", err)
	}
	return nil
}
