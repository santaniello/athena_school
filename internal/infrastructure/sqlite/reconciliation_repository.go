package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

const reconciliationProposalColumns = `id, action, status, candidate_snapshot, target_item_id, target_updated_at, reason, changes, created_at`

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

// GetByID returns the proposal with the given id, its EvidenceIDs
// populated from knowledge_reconciliation_evidence, or
// knowledge.ErrProposalNotFound if it does not exist.
func (r *ReconciliationRepository) GetByID(ctx context.Context, id string) (knowledge.ReconciliationProposal, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+reconciliationProposalColumns+` FROM knowledge_reconciliation_proposals WHERE id = ?`, id,
	)
	proposal, err := scanReconciliationProposal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return knowledge.ReconciliationProposal{}, knowledge.ErrProposalNotFound
	}
	if err != nil {
		return knowledge.ReconciliationProposal{}, fmt.Errorf("sqlite: getting reconciliation proposal: %w", err)
	}

	evidenceIDs, err := r.listEvidenceIDs(ctx, id)
	if err != nil {
		return knowledge.ReconciliationProposal{}, err
	}
	proposal.EvidenceIDs = evidenceIDs
	return proposal, nil
}

func (r *ReconciliationRepository) listEvidenceIDs(ctx context.Context, proposalID string) ([]string, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT evidence_id FROM knowledge_reconciliation_evidence WHERE proposal_id = ?`, proposalID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing reconciliation proposal evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	evidenceIDs := []string{}
	for rows.Next() {
		var evidenceID string
		if err := rows.Scan(&evidenceID); err != nil {
			return nil, fmt.Errorf("sqlite: scanning reconciliation proposal evidence: %w", err)
		}
		evidenceIDs = append(evidenceIDs, evidenceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating reconciliation proposal evidence: %w", err)
	}
	return evidenceIDs, nil
}

// ListByStatus returns every proposal currently at status, oldest first.
// EvidenceIDs is left empty on every entry — see knowledge.ReconciliationRepository.
func (r *ReconciliationRepository) ListByStatus(ctx context.Context, status string) ([]knowledge.ReconciliationProposal, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+reconciliationProposalColumns+` FROM knowledge_reconciliation_proposals WHERE status = ? ORDER BY created_at ASC, id ASC`,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing reconciliation proposals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	proposals := []knowledge.ReconciliationProposal{}
	for rows.Next() {
		proposal, err := scanReconciliationProposal(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning reconciliation proposal: %w", err)
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating reconciliation proposals: %w", err)
	}
	return proposals, nil
}

// UpdateStatus transitions id to status, stamping resolvedAt and
// overwriting reason, or returns knowledge.ErrProposalNotFound if it does
// not exist.
func (r *ReconciliationRepository) UpdateStatus(ctx context.Context, id, status, reason string, resolvedAt time.Time) error {
	result, err := execer(ctx, r.db).ExecContext(ctx,
		`UPDATE knowledge_reconciliation_proposals SET status = ?, reason = ?, resolved_at = ? WHERE id = ?`,
		status, reason, resolvedAt, id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: updating reconciliation proposal status: %w", err)
	}
	return requireRowAffected(result, knowledge.ErrProposalNotFound)
}

// CountByStatus returns how many proposals currently have the given status.
func (r *ReconciliationRepository) CountByStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM knowledge_reconciliation_proposals WHERE status = ?`, status,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: counting reconciliation proposals: %w", err)
	}
	return count, nil
}

func scanReconciliationProposal(scanner rowScanner) (knowledge.ReconciliationProposal, error) {
	var proposal knowledge.ReconciliationProposal
	var candidateSnapshot, changes string
	var targetItemID sql.NullString
	var targetUpdatedAt sql.NullTime
	err := scanner.Scan(
		&proposal.ID, &proposal.Action, &proposal.Status, &candidateSnapshot,
		&targetItemID, &targetUpdatedAt, &proposal.Reason, &changes, &proposal.CreatedAt,
	)
	if err != nil {
		return knowledge.ReconciliationProposal{}, err
	}
	if targetItemID.Valid {
		proposal.TargetItemID = targetItemID.String
	}
	if targetUpdatedAt.Valid {
		proposal.TargetUpdatedAt = targetUpdatedAt.Time
	}
	if err := json.Unmarshal([]byte(candidateSnapshot), &proposal.Candidate); err != nil {
		return knowledge.ReconciliationProposal{}, fmt.Errorf("decoding candidate snapshot for proposal %s: %w", proposal.ID, err)
	}
	if err := json.Unmarshal([]byte(changes), &proposal.Changes); err != nil {
		return knowledge.ReconciliationProposal{}, fmt.Errorf("decoding changes for proposal %s: %w", proposal.ID, err)
	}
	return proposal, nil
}
