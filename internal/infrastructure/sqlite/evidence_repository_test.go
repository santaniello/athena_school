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

func newTestEvidenceRepository(t *testing.T) (*EvidenceRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO knowledge_items
		(id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at)
		VALUES
		('item-a', 'Go', 'A', 'First.', '[]', '[]', '[]', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
		('item-b', 'Go', 'B', 'Second.', '[]', '[]', '[]', 'athena', 'draft', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return NewEvidenceRepository(db), db
}

func TestEvidenceRepository_GetOrCreateSharesSnapshotAndListsItForLinkedItems(t *testing.T) {
	// Given two Evidence values with the same origin and excerpt but different server-owned fields
	repository, _ := newTestEvidenceRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	first := domainknowledge.Evidence{
		ID: "evidence-first", OriginType: domainknowledge.OriginSessionMessage,
		OriginID: "message-1", SourceLabel: "Concurrency", Excerpt: "literal quote", CreatedAt: createdAt,
	}
	duplicate := domainknowledge.Evidence{
		ID: "evidence-duplicate", OriginType: domainknowledge.OriginSessionMessage,
		OriginID: "message-1", SourceLabel: "Changed label", Excerpt: "literal quote", CreatedAt: createdAt.Add(time.Hour),
	}

	// When creating each Evidence and linking the resolved snapshots to different Items
	resolvedFirst, firstErr := repository.GetOrCreate(ctx, first)
	resolvedDuplicate, duplicateErr := repository.GetOrCreate(ctx, duplicate)
	require.NoError(t, firstErr)
	require.NoError(t, duplicateErr)
	require.NoError(t, repository.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: "item-a", EvidenceID: resolvedFirst.ID}))
	require.NoError(t, repository.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: "item-b", EvidenceID: resolvedDuplicate.ID}))
	evidenceA, listAErr := repository.ListByItem(ctx, "item-a")
	evidenceB, listBErr := repository.ListByItem(ctx, "item-b")

	// Then the first immutable snapshot is reused and visible from both Items
	require.NoError(t, listAErr)
	require.NoError(t, listBErr)
	assert.Equal(t, first, resolvedFirst)
	assert.Equal(t, first, resolvedDuplicate)
	assert.Equal(t, []domainknowledge.Evidence{first}, evidenceA)
	assert.Equal(t, []domainknowledge.Evidence{first}, evidenceB)
}

func TestEvidenceRepository_DeleteUnreferencedPreservesSharedEvidenceUntilLastItemIsDeleted(t *testing.T) {
	// Given one Evidence snapshot linked to two Knowledge Items
	repository, db := newTestEvidenceRepository(t)
	ctx := context.Background()
	evidence, err := repository.GetOrCreate(ctx, domainknowledge.Evidence{
		ID: "evidence-1", OriginType: domainknowledge.OriginSessionMessage,
		OriginID: "message-1", SourceLabel: "Concurrency", Excerpt: "literal quote",
		CreatedAt: time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NoError(t, repository.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: "item-a", EvidenceID: evidence.ID}))
	require.NoError(t, repository.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: "item-b", EvidenceID: evidence.ID}))

	// When the first Item is deleted and unreferenced Evidence is cleaned
	_, err = db.Exec(`DELETE FROM knowledge_items WHERE id = 'item-a'`)
	require.NoError(t, err)
	require.NoError(t, repository.DeleteUnreferenced(ctx))

	// Then the shared Evidence remains available to the second Item
	remaining, err := repository.ListByItem(ctx, "item-b")
	require.NoError(t, err)
	assert.Equal(t, []domainknowledge.Evidence{evidence}, remaining)

	// When the final referencing Item is deleted and cleanup runs again
	_, err = db.Exec(`DELETE FROM knowledge_items WHERE id = 'item-b'`)
	require.NoError(t, err)
	require.NoError(t, repository.DeleteUnreferenced(ctx))

	// Then the now-unreferenced snapshot is deleted
	var evidenceCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM knowledge_evidence`).Scan(&evidenceCount))
	assert.Zero(t, evidenceCount)
}

func TestEvidenceRepository_SavingAnItemWithItsEvidenceIsAtomic_aFailureLeavesNeitherBehind(t *testing.T) {
	// Given a Knowledge Item repository and an Evidence repository sharing
	// one real transaction, and a trigger forcing the item-evidence link to
	// fail — proving rollback for real, not just through a mocked Transactor
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TRIGGER always_fail_item_evidence_link
		BEFORE INSERT ON knowledge_item_evidence
		BEGIN SELECT RAISE(FAIL, 'boom'); END`)
	require.NoError(t, err)
	items := NewKnowledgeRepository(db)
	evidence := NewEvidenceRepository(db)
	transactor := NewSQLTransactor(db)
	item := testItem("item-atomic", "Go", domainknowledge.StatusDraft)
	ctx := context.Background()

	// When saving the Item and linking its Evidence inside one transaction
	// that then hits the failing trigger
	txErr := transactor.WithinTx(ctx, func(ctx context.Context) error {
		if err := items.Save(ctx, item); err != nil {
			return err
		}
		resolved, err := evidence.GetOrCreate(ctx, domainknowledge.Evidence{
			ID: "evidence-atomic", OriginType: domainknowledge.OriginSessionMessage,
			OriginID: "message-1", SourceLabel: "Go", Excerpt: "literal quote",
			CreatedAt: time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC),
		})
		if err != nil {
			return err
		}
		return evidence.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: item.ID, EvidenceID: resolved.ID})
	})

	// Then the whole transaction rolled back: neither the Item nor the
	// Evidence snapshot — created earlier in the same transaction — survived
	require.ErrorContains(t, txErr, "boom")
	_, getErr := items.GetByID(ctx, "item-atomic")
	assert.ErrorIs(t, getErr, domainknowledge.ErrItemNotFound)
	var evidenceCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM knowledge_evidence`).Scan(&evidenceCount))
	assert.Zero(t, evidenceCount)
}
