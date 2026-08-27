package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// EvidenceRepository is the SQLite-backed knowledge.EvidenceRepository.
type EvidenceRepository struct {
	db *sql.DB
}

// NewEvidenceRepository creates an EvidenceRepository backed by db.
func NewEvidenceRepository(db *sql.DB) *EvidenceRepository {
	return &EvidenceRepository{db: db}
}

// GetOrCreate stores evidence or returns the immutable snapshot already
// sharing its origin and excerpt.
func (r *EvidenceRepository) GetOrCreate(ctx context.Context, evidence knowledge.Evidence) (knowledge.Evidence, error) {
	_, err := execer(ctx, r.db).ExecContext(ctx, `INSERT INTO knowledge_evidence
		(id, origin_type, origin_id, source_label, excerpt, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (origin_type, origin_id, excerpt) DO NOTHING`,
		evidence.ID, evidence.OriginType, evidence.OriginID, evidence.SourceLabel, evidence.Excerpt, evidence.CreatedAt,
	)
	if err != nil {
		return knowledge.Evidence{}, fmt.Errorf("sqlite: creating knowledge evidence: %w", err)
	}

	resolved, err := scanEvidence(execer(ctx, r.db).QueryRowContext(ctx, `SELECT
		id, origin_type, origin_id, source_label, excerpt, created_at
		FROM knowledge_evidence
		WHERE origin_type = ? AND origin_id = ? AND excerpt = ?`,
		evidence.OriginType, evidence.OriginID, evidence.Excerpt,
	))
	if err != nil {
		return knowledge.Evidence{}, fmt.Errorf("sqlite: resolving knowledge evidence: %w", err)
	}
	return resolved, nil
}

// LinkToItem attaches an Evidence snapshot to a Knowledge Item.
func (r *EvidenceRepository) LinkToItem(ctx context.Context, link knowledge.ItemEvidence) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO knowledge_item_evidence (item_id, evidence_id) VALUES (?, ?)`,
		link.ItemID, link.EvidenceID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: linking knowledge evidence: %w", err)
	}
	return nil
}

// ListByItem returns the immutable Evidence snapshots attached to itemID.
func (r *EvidenceRepository) ListByItem(ctx context.Context, itemID string) ([]knowledge.Evidence, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, `SELECT
		e.id, e.origin_type, e.origin_id, e.source_label, e.excerpt, e.created_at
		FROM knowledge_evidence e
		JOIN knowledge_item_evidence ie ON ie.evidence_id = e.id
		WHERE ie.item_id = ?
		ORDER BY e.created_at ASC, e.id ASC`, itemID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing knowledge evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	evidence := []knowledge.Evidence{}
	for rows.Next() {
		entry, scanErr := scanEvidence(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("sqlite: scanning knowledge evidence: %w", scanErr)
		}
		evidence = append(evidence, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating knowledge evidence: %w", err)
	}
	return evidence, nil
}

// DeleteUnreferenced removes snapshots that no Knowledge Item uses.
func (r *EvidenceRepository) DeleteUnreferenced(ctx context.Context) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM knowledge_evidence
		WHERE NOT EXISTS (
			SELECT 1 FROM knowledge_item_evidence ie
			WHERE ie.evidence_id = knowledge_evidence.id
		)`)
	if err != nil {
		return fmt.Errorf("sqlite: deleting unreferenced knowledge evidence: %w", err)
	}
	return nil
}

type evidenceScanner interface {
	Scan(dest ...any) error
}

func scanEvidence(scanner evidenceScanner) (knowledge.Evidence, error) {
	var evidence knowledge.Evidence
	err := scanner.Scan(
		&evidence.ID,
		&evidence.OriginType,
		&evidence.OriginID,
		&evidence.SourceLabel,
		&evidence.Excerpt,
		&evidence.CreatedAt,
	)
	return evidence, err
}
