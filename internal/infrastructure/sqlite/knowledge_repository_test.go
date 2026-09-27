package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

func newTestKnowledgeRepository(t *testing.T) *KnowledgeRepository {
	t.Helper()
	repo, _ := newTestKnowledgeRepositoryWithDB(t)
	return repo
}

// newTestKnowledgeRepositoryWithDB also returns the raw *sql.DB, for tests
// that need to inspect normalized_concept — a column Item does not expose,
// since it is derived storage the domain layer never reads directly.
func newTestKnowledgeRepositoryWithDB(t *testing.T) (*KnowledgeRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, testSessionID)
	return NewKnowledgeRepository(db), db
}

func testItem(id, topic string) knowledge.Item {
	now := time.Now().UTC().Truncate(time.Second)
	return knowledge.Item{
		ID:              id,
		SessionID:       testSessionID,
		Topic:           topic,
		Concept:         "Concept " + id,
		Definition:      "Definition " + id,
		Properties:      []string{"prop-1", "prop-2", "prop-3"},
		TradeOffs:       []string{"trade-off-1"},
		RelatedConcepts: []string{"related-1"},
		Source:          knowledge.SourceImportedDoc,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestKnowledgeRepository_Save_andGetByID_roundTripsEveryField(t *testing.T) {
	// Given a repository and a fully populated item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency")

	// When saving it and reading it back
	require.NoError(t, repo.Save(ctx, item))
	stored, err := repo.GetByID(ctx, "item-1")

	// Then every field round-trips exactly
	require.NoError(t, err)
	assert.Equal(t, item.ID, stored.ID)
	assert.Equal(t, item.Topic, stored.Topic)
	assert.Equal(t, item.Concept, stored.Concept)
	assert.Equal(t, item.Definition, stored.Definition)
	assert.Equal(t, item.Properties, stored.Properties)
	assert.Equal(t, item.TradeOffs, stored.TradeOffs)
	assert.Equal(t, item.RelatedConcepts, stored.RelatedConcepts)
	assert.Equal(t, item.Source, stored.Source)
	assert.Equal(t, item.Source, stored.Source)
	assert.Equal(t, item.CreatedAt, stored.CreatedAt)
	assert.Equal(t, item.UpdatedAt, stored.UpdatedAt)
}

func TestKnowledgeRepository_Save_roundTripsNilSlicesAsEmpty(t *testing.T) {
	// Given an item with nil list fields
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency")
	item.Properties = nil
	item.TradeOffs = nil
	item.RelatedConcepts = nil

	// When saving it and reading it back
	require.NoError(t, repo.Save(ctx, item))
	stored, err := repo.GetByID(ctx, "item-1")

	// Then the nil slices round-trip to empty, never nil
	require.NoError(t, err)
	assert.Equal(t, []string{}, stored.Properties)
	assert.Equal(t, []string{}, stored.TradeOffs)
	assert.Equal(t, []string{}, stored.RelatedConcepts)
}

func TestKnowledgeRepository_Save_roundTripsEmptySlicesAsEmpty(t *testing.T) {
	// Given an item with explicitly empty (non-nil) list fields
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency")
	item.Properties = []string{}
	item.TradeOffs = []string{}
	item.RelatedConcepts = []string{}

	// When saving it and reading it back
	require.NoError(t, repo.Save(ctx, item))
	stored, err := repo.GetByID(ctx, "item-1")

	// Then the slices round-trip to empty, never nil
	require.NoError(t, err)
	assert.Equal(t, []string{}, stored.Properties)
	assert.Equal(t, []string{}, stored.TradeOffs)
	assert.Equal(t, []string{}, stored.RelatedConcepts)
}

func TestKnowledgeRepository_GetByID_returnsErrItemNotFound_whenMissing(t *testing.T) {
	// Given a repository with no matching item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()

	// When fetching an item that does not exist
	_, err := repo.GetByID(ctx, "missing")

	// Then it fails with ErrItemNotFound
	assert.ErrorIs(t, err, knowledge.ErrItemNotFound)
}

func TestKnowledgeRepository_GetByID_returnsError_whenPropertiesColumnHasInvalidJSON(t *testing.T) {
	// Given an item whose properties column was corrupted (e.g. by a bug
	// or manual edit) into something that isn't valid JSON
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency")
	require.NoError(t, repo.Save(ctx, item))
	_, execErr := repo.db.ExecContext(ctx,
		`UPDATE knowledge_items SET properties = 'not json' WHERE id = ?`, "item-1")
	require.NoError(t, execErr)

	// When reading it back
	_, err := repo.GetByID(ctx, "item-1")

	// Then it fails instead of silently returning an empty slice
	assert.Error(t, err)
}

func TestKnowledgeRepository_Update_persistsChanges(t *testing.T) {
	// Given a saved item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency")
	require.NoError(t, repo.Save(ctx, item))

	// When updating its fields
	item.Definition = "A new definition"
	item.Topic = "Rust"
	err := repo.Update(ctx, item)

	// Then the changes are persisted
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, "item-1")
	require.NoError(t, getErr)
	assert.Equal(t, "A new definition", stored.Definition)
	assert.Equal(t, "Rust", stored.Topic)
}

func TestKnowledgeRepository_Update_returnsErrItemNotFound_whenMissing(t *testing.T) {
	// Given a repository with no matching item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("missing", "Go Concurrency")

	// When updating an item that does not exist
	err := repo.Update(ctx, item)

	// Then it fails with ErrItemNotFound
	assert.ErrorIs(t, err, knowledge.ErrItemNotFound)
}

func TestKnowledgeRepository_Delete_removesItem(t *testing.T) {
	// Given a saved item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency")))

	// When deleting it
	err := repo.Delete(ctx, "item-1")

	// Then it no longer exists
	require.NoError(t, err)
	_, getErr := repo.GetByID(ctx, "item-1")
	assert.ErrorIs(t, getErr, knowledge.ErrItemNotFound)
}

func TestKnowledgeRepository_Delete_returnsErrItemNotFound_whenMissing(t *testing.T) {
	// Given a repository with no matching item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()

	// When deleting an item that does not exist
	err := repo.Delete(ctx, "missing")

	// Then it fails with ErrItemNotFound
	assert.ErrorIs(t, err, knowledge.ErrItemNotFound)
}

func TestKnowledgeRepository_Save_participatesInCallerTransaction(t *testing.T) {
	// Given a repository and a transactor sharing the same database
	db := newTestDB(t)
	seedSession(t, db, testSessionID)
	repo := NewKnowledgeRepository(db)
	transactor := NewSQLTransactor(db)
	item := testItem("item-1", "Go")
	boom := errors.New("boom")

	// When Save runs inside a transaction that is then rolled back
	txErr := transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := repo.Save(ctx, item); err != nil {
			return err
		}
		return boom
	})

	// Then Save's write never became visible
	require.ErrorIs(t, txErr, boom)
	_, getErr := repo.GetByID(context.Background(), "item-1")
	assert.ErrorIs(t, getErr, knowledge.ErrItemNotFound)
}
