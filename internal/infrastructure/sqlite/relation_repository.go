package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// RelationRepository is the SQLite-backed knowledge.RelationRepository.
type RelationRepository struct {
	db *sql.DB
}

// NewRelationRepository creates a RelationRepository backed by db.
func NewRelationRepository(db *sql.DB) *RelationRepository {
	return &RelationRepository{db: db}
}

// Save persists relation. ON CONFLICT DO NOTHING makes it idempotent for
// the same canonical (from, to, type) triple, mirroring the table's
// composite primary key.
func (r *RelationRepository) Save(ctx context.Context, relation knowledge.Relation) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO knowledge_item_relations (from_item_id, to_item_id, relation_type, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (from_item_id, to_item_id, relation_type) DO NOTHING`,
		relation.FromItemID, relation.ToItemID, relation.Type, relation.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving knowledge item relation: %w", err)
	}
	return nil
}
