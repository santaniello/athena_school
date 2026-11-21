package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// EvidenceRepository is the SQLite-backed implementation of
// domainstudy.EvidenceRepository. It shares conceptTimeLayout with the
// concept state, so created_at orders as text.
type EvidenceRepository struct {
	db *sql.DB
}

// NewEvidenceRepository creates an EvidenceRepository backed by db. db must
// already have its migrations applied (see Open).
func NewEvidenceRepository(db *sql.DB) *EvidenceRepository {
	return &EvidenceRepository{db: db}
}

// Add stores one proof. Its concept row and both messages must exist.
func (r *EvidenceRepository) Add(ctx context.Context, evidence domainstudy.Evidence) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `
		INSERT INTO concept_evidence
			(session_id, concept, assessment, description, assisted, user_message_id, reply_message_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		evidence.SessionID, evidence.Concept, string(evidence.Assessment), evidence.Description,
		evidence.Assisted, evidence.UserMessageID, evidence.ReplyMessageID,
		evidence.CreatedAt.UTC().Format(conceptTimeLayout),
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving concept evidence: %w", err)
	}
	return nil
}

// TrimOldest keeps only the keep most recent proofs of the concept.
func (r *EvidenceRepository) TrimOldest(ctx context.Context, sessionID, concept string, keep int) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `
		DELETE FROM concept_evidence WHERE session_id = ? AND concept = ? AND id NOT IN (
			SELECT id FROM concept_evidence WHERE session_id = ? AND concept = ?
			ORDER BY created_at DESC, id DESC LIMIT ?
		)`, sessionID, concept, sessionID, concept, keep)
	if err != nil {
		return fmt.Errorf("sqlite: trimming concept evidence: %w", err)
	}
	return nil
}

// ListBySession returns the session's proofs, oldest first.
func (r *EvidenceRepository) ListBySession(ctx context.Context, sessionID string) ([]domainstudy.Evidence, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, `
		SELECT session_id, concept, assessment, description, assisted, user_message_id, reply_message_id, created_at
		FROM concept_evidence WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing concept evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []domainstudy.Evidence
	for rows.Next() {
		var evidence domainstudy.Evidence
		var assessment, createdAt string
		if err := rows.Scan(&evidence.SessionID, &evidence.Concept, &assessment, &evidence.Description,
			&evidence.Assisted, &evidence.UserMessageID, &evidence.ReplyMessageID, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning concept evidence: %w", err)
		}
		evidence.Assessment = domainstudy.Assessment(assessment)
		if evidence.CreatedAt, err = time.Parse(conceptTimeLayout, createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: parsing concept evidence time: %w", err)
		}
		list = append(list, evidence)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: listing concept evidence: %w", err)
	}
	return list, nil
}
