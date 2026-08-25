package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

const testUnindexedEmbeddingModel = "text-embedding-3-small"

// currentChunkFor returns a chunk matching item exactly as ListCurrent
// would consider fresh: same topic/status, the item's current UpdatedAt,
// and the current embedding model.
func currentChunkFor(item knowledge.Item, chunkID string) knowledge.Chunk {
	return knowledge.Chunk{
		ID: chunkID, Source: knowledge.SourceAthena, Topic: item.Topic, Status: item.Status,
		ItemID: item.ID, Content: "content", Embedding: []float32{0.1, 0.2},
		EmbeddingModel: testUnindexedEmbeddingModel, ItemUpdatedAt: item.UpdatedAt,
		CreatedAt: item.UpdatedAt,
	}
}

func TestKnowledgeRepository_CountUnindexed_countsAnAthenaItemWithNoChunkAtAll(t *testing.T) {
	// Given an athena item with no chunk
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusDraft)
	require.NoError(t, items.Save(ctx, item))

	// When counting unindexed items
	count, err := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then it is counted
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	_ = chunks
}

func TestKnowledgeRepository_CountUnindexed_excludesAnItemWithACurrentChunk(t *testing.T) {
	// Given an athena item with a chunk matching it exactly
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusApproved)
	require.NoError(t, items.Save(ctx, item))
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{currentChunkFor(item, "chunk-1")}))

	// When counting unindexed items
	count, err := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then it is not counted
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestKnowledgeRepository_CountUnindexed_countsAnItemWhoseChunkItemUpdatedAtIsStale(t *testing.T) {
	// Given an athena item whose chunk was indexed before its latest edit
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusApproved)
	require.NoError(t, items.Save(ctx, item))
	staleChunk := currentChunkFor(item, "chunk-1")
	staleChunk.ItemUpdatedAt = item.UpdatedAt.Add(-time.Hour)
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{staleChunk}))

	// When counting unindexed items
	count, err := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then it is counted — an update committed and then failed to re-embed
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestKnowledgeRepository_CountUnindexed_countsAnItemWhoseChunkStatusDiffers(t *testing.T) {
	// Given an athena item that was approved after its chunk was indexed as a draft
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusApproved)
	require.NoError(t, items.Save(ctx, item))
	staleChunk := currentChunkFor(item, "chunk-1")
	staleChunk.Status = knowledge.StatusDraft
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{staleChunk}))

	// When counting unindexed items
	count, err := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then it is counted
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestKnowledgeRepository_CountUnindexed_countsAnItemWhoseChunkUsesAnOlderEmbeddingModel(t *testing.T) {
	// Given an athena item indexed under a since-changed embedding model
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusApproved)
	require.NoError(t, items.Save(ctx, item))
	staleChunk := currentChunkFor(item, "chunk-1")
	staleChunk.EmbeddingModel = "text-embedding-3-large"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{staleChunk}))

	// When counting unindexed items against the new current model
	count, err := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then it is counted — every item becomes eligible after a model change
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestKnowledgeRepository_CountUnindexed_excludesImportedDocShadowItems(t *testing.T) {
	// Given an imported-note shadow Item with no chunk — its freshness is
	// governed by ingested_files/2.3, never by item_updated_at, so it must
	// never be counted here regardless of chunk state
	_, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusApproved)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))

	// When counting unindexed items
	count, err := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then it is not counted
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestKnowledgeRepository_ListUnindexed_returnsOnlyUnindexedAthenaItems_oldestFirst(t *testing.T) {
	// Given one current athena item, one unindexed athena item, and one
	// imported-doc item with no chunk
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	current := testItem("item-current", "Go", knowledge.StatusApproved)
	current.CreatedAt, current.UpdatedAt = now.Add(-2*time.Hour), now.Add(-2*time.Hour)
	require.NoError(t, items.Save(ctx, current))
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{currentChunkFor(current, "chunk-current")}))

	unindexed := testItem("item-unindexed", "Rust", knowledge.StatusDraft)
	unindexed.CreatedAt, unindexed.UpdatedAt = now.Add(-time.Hour), now.Add(-time.Hour)
	require.NoError(t, items.Save(ctx, unindexed))

	newer := testItem("item-unindexed-newer", "Elixir", knowledge.StatusDraft)
	newer.CreatedAt, newer.UpdatedAt = now.Add(-30*time.Minute), now.Add(-30*time.Minute)
	require.NoError(t, items.Save(ctx, newer))

	imported := testItem("item-imported", "Python", knowledge.StatusApproved)
	imported.Source = knowledge.SourceImportedDoc
	imported.CreatedAt, imported.UpdatedAt = now, now
	require.NoError(t, items.Save(ctx, imported))

	// When listing unindexed items
	got, err := items.ListUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then only the unindexed athena items come back, oldest first
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, []string{"item-unindexed", "item-unindexed-newer"},
		[]string{got[0].ID, got[1].ID})
}

func TestKnowledgeRepository_ListUnindexed_excludesAnItemWithACurrentChunkAlongsideAStaleOne(t *testing.T) {
	// Given an athena item that carries both its current chunk and a
	// leftover stale chunk (e.g. a prior VectorStore eviction failure that
	// left the old row behind) — the LEFT JOIN this query used to run
	// would match both rows and could double-count or misclassify the item
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	item := testItem("item-1", "Go", knowledge.StatusApproved)
	require.NoError(t, items.Save(ctx, item))
	stale := currentChunkFor(item, "chunk-stale")
	stale.ItemUpdatedAt = item.UpdatedAt.Add(-time.Hour)
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{
		currentChunkFor(item, "chunk-current"),
		stale,
	}))

	// When counting and listing unindexed items
	count, countErr := items.CountUnindexed(ctx, testUnindexedEmbeddingModel)
	got, listErr := items.ListUnindexed(ctx, testUnindexedEmbeddingModel)

	// Then the item is excluded from both, exactly once
	require.NoError(t, countErr)
	require.NoError(t, listErr)
	assert.Equal(t, 0, count)
	assert.Empty(t, got)
}
