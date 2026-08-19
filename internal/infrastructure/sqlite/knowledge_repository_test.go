package sqlite

import (
	"context"
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
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewKnowledgeRepository(db)
}

func testItem(id, topic, status string) knowledge.Item {
	now := time.Now().UTC().Truncate(time.Second)
	return knowledge.Item{
		ID:              id,
		Topic:           topic,
		Concept:         "Concept " + id,
		Definition:      "Definition " + id,
		Properties:      []string{"prop-1", "prop-2", "prop-3"},
		TradeOffs:       []string{"trade-off-1"},
		RelatedConcepts: []string{"related-1"},
		Source:          knowledge.SourceAthena,
		Status:          status,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestKnowledgeRepository_Save_andGetByID_roundTripsEveryField(t *testing.T) {
	// Given a repository and a fully populated item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency", knowledge.StatusDraft)

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
	assert.Equal(t, item.Status, stored.Status)
	assert.Equal(t, item.CreatedAt, stored.CreatedAt)
	assert.Equal(t, item.UpdatedAt, stored.UpdatedAt)
}

func TestKnowledgeRepository_Save_roundTripsNilSlicesAsEmpty(t *testing.T) {
	// Given an item with nil list fields
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency", knowledge.StatusDraft)
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
	item := testItem("item-1", "Go Concurrency", knowledge.StatusDraft)
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
	item := testItem("item-1", "Go Concurrency", knowledge.StatusDraft)
	require.NoError(t, repo.Save(ctx, item))
	_, execErr := repo.db.ExecContext(ctx,
		`UPDATE knowledge_items SET properties = 'not json' WHERE id = ?`, "item-1")
	require.NoError(t, execErr)

	// When reading it back
	_, err := repo.GetByID(ctx, "item-1")

	// Then it fails instead of silently returning an empty slice
	assert.Error(t, err)
}

func TestKnowledgeRepository_FindByTopic_returnsOnlyItemsOfThatTopic(t *testing.T) {
	// Given items across two topics
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Go Concurrency", knowledge.StatusApproved)))
	require.NoError(t, repo.Save(ctx, testItem("item-3", "Java Generics", knowledge.StatusDraft)))

	// When finding items by topic
	items, err := repo.FindByTopic(ctx, "Go Concurrency")

	// Then only that topic's items are returned
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "item-1", items[0].ID)
	assert.Equal(t, "item-2", items[1].ID)
}

func TestKnowledgeRepository_List_returnsEverything_whenFilterIsEmpty(t *testing.T) {
	// Given items across topics and statuses
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Java Generics", knowledge.StatusApproved)))

	// When listing with no filter
	items, err := repo.List(ctx, knowledge.Filter{})

	// Then every item is returned
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestKnowledgeRepository_List_honoursTopicFilter(t *testing.T) {
	// Given items across two topics
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Java Generics", knowledge.StatusDraft)))

	// When listing filtered by topic
	items, err := repo.List(ctx, knowledge.Filter{Topic: "Go Concurrency"})

	// Then only matching items are returned
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "item-1", items[0].ID)
}

func TestKnowledgeRepository_List_honoursStatusFilter(t *testing.T) {
	// Given items with different statuses
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Go Concurrency", knowledge.StatusApproved)))

	// When listing filtered by status
	items, err := repo.List(ctx, knowledge.Filter{Status: knowledge.StatusApproved})

	// Then only matching items are returned
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "item-2", items[0].ID)
}

func TestKnowledgeRepository_List_honoursCombinedTopicAndStatusFilter(t *testing.T) {
	// Given items across topics and statuses
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Go Concurrency", knowledge.StatusApproved)))
	require.NoError(t, repo.Save(ctx, testItem("item-3", "Java Generics", knowledge.StatusApproved)))

	// When listing filtered by both topic and status
	items, err := repo.List(ctx, knowledge.Filter{Topic: "Go Concurrency", Status: knowledge.StatusApproved})

	// Then only the item matching both constraints is returned
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "item-2", items[0].ID)
}

func TestKnowledgeRepository_List_returnsItemsOldestFirst(t *testing.T) {
	// Given items saved with increasing timestamps
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second)
	older := testItem("item-older", "Go Concurrency", knowledge.StatusDraft)
	older.CreatedAt = base
	newer := testItem("item-newer", "Go Concurrency", knowledge.StatusDraft)
	newer.CreatedAt = base.Add(time.Hour)
	require.NoError(t, repo.Save(ctx, newer))
	require.NoError(t, repo.Save(ctx, older))

	// When listing
	items, err := repo.List(ctx, knowledge.Filter{})

	// Then the oldest item comes first
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "item-older", items[0].ID)
	assert.Equal(t, "item-newer", items[1].ID)
}

func TestKnowledgeRepository_List_tieBreaksByID_whenCreatedAtCollides(t *testing.T) {
	// Given two items sharing the exact same timestamp
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	same := time.Now().UTC().Truncate(time.Second)
	first := testItem("item-b", "Go Concurrency", knowledge.StatusDraft)
	first.CreatedAt = same
	second := testItem("item-a", "Go Concurrency", knowledge.StatusDraft)
	second.CreatedAt = same
	require.NoError(t, repo.Save(ctx, first))
	require.NoError(t, repo.Save(ctx, second))

	// When listing
	items, err := repo.List(ctx, knowledge.Filter{})

	// Then items are ordered by id as a deterministic tiebreak
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "item-a", items[0].ID)
	assert.Equal(t, "item-b", items[1].ID)
}

func TestKnowledgeRepository_ListTopics_returnsDistinctTopicsAlphabetically(t *testing.T) {
	// Given items across topics, with one topic repeated
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Java Generics", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-3", "Go Concurrency", knowledge.StatusApproved)))

	// When listing topics
	topics, err := repo.ListTopics(ctx)

	// Then distinct topics are returned, alphabetically
	require.NoError(t, err)
	assert.Equal(t, []string{"Go Concurrency", "Java Generics"}, topics)
}

func TestKnowledgeRepository_CountByStatus_returnsCountPerStatus(t *testing.T) {
	// Given items across statuses
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-2", "Go Concurrency", knowledge.StatusDraft)))
	require.NoError(t, repo.Save(ctx, testItem("item-3", "Go Concurrency", knowledge.StatusApproved)))

	// When counting by status
	draftCount, draftErr := repo.CountByStatus(ctx, knowledge.StatusDraft)
	approvedCount, approvedErr := repo.CountByStatus(ctx, knowledge.StatusApproved)

	// Then each status is counted correctly
	require.NoError(t, draftErr)
	require.NoError(t, approvedErr)
	assert.Equal(t, 2, draftCount)
	assert.Equal(t, 1, approvedCount)
}

func TestKnowledgeRepository_Update_persistsChanges(t *testing.T) {
	// Given a saved item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("item-1", "Go Concurrency", knowledge.StatusDraft)
	require.NoError(t, repo.Save(ctx, item))

	// When updating its fields
	item.Definition = "A new definition"
	item.Status = knowledge.StatusApproved
	err := repo.Update(ctx, item)

	// Then the changes are persisted
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, "item-1")
	require.NoError(t, getErr)
	assert.Equal(t, "A new definition", stored.Definition)
	assert.Equal(t, knowledge.StatusApproved, stored.Status)
}

func TestKnowledgeRepository_Update_returnsErrItemNotFound_whenMissing(t *testing.T) {
	// Given a repository with no matching item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	item := testItem("missing", "Go Concurrency", knowledge.StatusDraft)

	// When updating an item that does not exist
	err := repo.Update(ctx, item)

	// Then it fails with ErrItemNotFound
	assert.ErrorIs(t, err, knowledge.ErrItemNotFound)
}

func TestKnowledgeRepository_Delete_removesItem(t *testing.T) {
	// Given a saved item
	repo := newTestKnowledgeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, testItem("item-1", "Go Concurrency", knowledge.StatusDraft)))

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
	repo := NewKnowledgeRepository(db)
	transactor := NewSQLTransactor(db)
	item := testItem("item-1", "Go", knowledge.StatusDraft)
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
