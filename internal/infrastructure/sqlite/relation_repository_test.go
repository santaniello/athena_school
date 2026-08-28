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

func newTestRelationRepository(t *testing.T) (*RelationRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO knowledge_items
		(id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at)
		VALUES
		('item-a', 'Distributed Systems', 'CAP theorem', 'Pick two.', '[]', '[]', '[]', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('item-b', 'Distributed Systems', 'CAP Theorem (Brewer theorem)', 'Pick two of three.', '[]', '[]', '[]', 'athena', 'approved', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return NewRelationRepository(db), db
}

func TestRelationRepository_SaveIsIdempotentForTheSameCanonicalRelation(t *testing.T) {
	// Given the canonical relation between two items
	repository, db := newTestRelationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	relation := domainknowledge.CanonicalRelation("item-a", "item-b", domainknowledge.RelationRelated, createdAt)

	// When saving it twice — e.g. the user re-applies the same relate proposal
	err1 := repository.Save(ctx, relation)
	err2 := repository.Save(ctx, relation)

	// Then only one row exists, canonically ordered
	require.NoError(t, err1)
	require.NoError(t, err2)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM knowledge_item_relations`).Scan(&count))
	assert.Equal(t, 1, count)
	var fromItemID, toItemID string
	require.NoError(t, db.QueryRow(`SELECT from_item_id, to_item_id FROM knowledge_item_relations`).Scan(&fromItemID, &toItemID))
	assert.Equal(t, "item-a", fromItemID)
	assert.Equal(t, "item-b", toItemID)
}
