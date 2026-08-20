package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

const testEmbeddingModel = "text-embedding-3-small"

func newTestChunkRepository(t *testing.T) *ChunkRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewChunkRepository(db)
}

// newTestChunkAndItemRepositories returns a ChunkRepository and a
// KnowledgeRepository sharing one database, plus the raw *sql.DB so a test
// can force a database-wide failure (e.g. by closing it early).
func newTestChunkAndItemRepositories(t *testing.T) (*ChunkRepository, *KnowledgeRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewChunkRepository(db), NewKnowledgeRepository(db), db
}

// testItem is a matching owner for testChunk: same topic/status, source
// athena by default (tests override Source for imported_doc scenarios).
func testItemAt(id, topic, status string, updatedAt time.Time) knowledge.Item {
	return knowledge.Item{
		ID: id, Topic: topic, Concept: "Concept", Definition: "A definition.",
		Source: knowledge.SourceAthena, Status: status,
		CreatedAt: updatedAt, UpdatedAt: updatedAt,
	}
}

func testChunk(id, filePath string, createdAt time.Time) knowledge.Chunk {
	return knowledge.Chunk{
		ID:             id,
		Source:         knowledge.SourceImportedDoc,
		Topic:          "Go",
		Status:         knowledge.StatusApproved,
		ItemID:         "item-" + id,
		SourcePath:     "/abs/" + filePath,
		FilePath:       filePath,
		Heading:        "Intro",
		Content:        "Content for " + id,
		Embedding:      []float32{0.1, 0.2, 0.3},
		EmbeddingModel: "text-embedding-3-small",
		CreatedAt:      createdAt,
	}
}

func TestChunkRepository_SaveAll_thenListAll_roundTripsEveryField(t *testing.T) {
	// Given a repository and one chunk with an athena Item's UpdatedAt set
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	chunk := testChunk("chunk-1", "notes/go.md", time.Now().UTC().Truncate(time.Second))
	chunk.ItemUpdatedAt = time.Now().UTC().Truncate(time.Second)

	// When saving then listing it
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{chunk}))
	got, err := repo.ListAll(ctx)

	// Then every field round-trips, including the embedding and item_updated_at
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, chunk, got[0])
}

func TestChunkRepository_SaveAll_roundTripsZeroItemUpdatedAt_asZeroTime(t *testing.T) {
	// Given an imported-file chunk with no ItemUpdatedAt (zero value)
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	chunk := testChunk("chunk-1", "notes/go.md", time.Now().UTC().Truncate(time.Second))

	// When saving then listing it
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{chunk}))
	got, err := repo.ListAll(ctx)

	// Then ItemUpdatedAt round-trips as the zero value, not some sentinel
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.True(t, got[0].ItemUpdatedAt.IsZero())
}

func TestChunkRepository_ListAll_returnsChunksOldestFirst(t *testing.T) {
	// Given two chunks saved with distinct created_at timestamps
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	older := testChunk("chunk-old", "notes/a.md", time.Now().UTC().Add(-time.Hour).Truncate(time.Second))
	newer := testChunk("chunk-new", "notes/b.md", time.Now().UTC().Truncate(time.Second))
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{newer, older}))

	// When listing all chunks
	got, err := repo.ListAll(ctx)

	// Then they come back oldest first
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "chunk-old", got[0].ID)
	assert.Equal(t, "chunk-new", got[1].ID)
}

func TestChunkRepository_DeleteBySourcePath_removesOnlyThatSourcesChunks_andReturnsRemovedIDs(t *testing.T) {
	// Given chunks from two different sources
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{
		testChunk("chunk-a1", "notes/a.md", now),
		testChunk("chunk-a2", "notes/a.md", now),
		testChunk("chunk-b1", "notes/b.md", now),
	}))

	// When deleting by one source's path
	removedIDs, err := repo.DeleteBySourcePath(ctx, "/abs/notes/a.md")

	// Then only that source's chunks are gone, and their IDs are returned
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"chunk-a1", "chunk-a2"}, removedIDs)
	got, listErr := repo.ListAll(ctx)
	require.NoError(t, listErr)
	require.Len(t, got, 1)
	assert.Equal(t, "chunk-b1", got[0].ID)
}

func TestChunkRepository_DeleteBySourcePath_isNoOp_whenNothingMatches(t *testing.T) {
	// Given a repository with no chunks at all
	repo := newTestChunkRepository(t)
	ctx := context.Background()

	// When deleting by a source path that was never ingested
	removedIDs, err := repo.DeleteBySourcePath(ctx, "/abs/notes/never-imported.md")

	// Then it succeeds without error and returns no IDs
	require.NoError(t, err)
	assert.Empty(t, removedIDs)
}

func TestChunkRepository_DeleteBySourcePath_targetsOnlyOneSource_whenTwoRootsShareTheSameRelativeName(t *testing.T) {
	// Given two chunks with the same display FilePath but distinct
	// canonical SourcePath identities — e.g. /course-a/notes.md and
	// /course-b/notes.md — proving they coexist rather than colliding
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	courseA := testChunk("chunk-a", "notes.md", now)
	courseA.SourcePath = "/course-a/notes.md"
	courseB := testChunk("chunk-b", "notes.md", now)
	courseB.SourcePath = "/course-b/notes.md"
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{courseA, courseB}))

	// When deleting by only one of the two sources
	removedIDs, err := repo.DeleteBySourcePath(ctx, "/course-a/notes.md")

	// Then only that source's chunk is gone; the other survives untouched
	require.NoError(t, err)
	assert.Equal(t, []string{"chunk-a"}, removedIDs)
	got, listErr := repo.ListAll(ctx)
	require.NoError(t, listErr)
	require.Len(t, got, 1)
	assert.Equal(t, "chunk-b", got[0].ID)
	assert.Equal(t, "/course-b/notes.md", got[0].SourcePath)
}

func TestChunkRepository_DeleteByItemID_removesOnlyThatItemsChunks_andReturnsRemovedIDs(t *testing.T) {
	// Given chunks owned by two different items
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	a1 := testChunk("chunk-a1", "notes/a.md", now)
	a1.ItemID = "item-a"
	b1 := testChunk("chunk-b1", "notes/b.md", now)
	b1.ItemID = "item-b"
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{a1, b1}))

	// When deleting by one item's ID
	removedIDs, err := repo.DeleteByItemID(ctx, "item-a")

	// Then only that item's chunks are gone, and their IDs are returned
	require.NoError(t, err)
	assert.Equal(t, []string{"chunk-a1"}, removedIDs)
	got, listErr := repo.ListAll(ctx)
	require.NoError(t, listErr)
	require.Len(t, got, 1)
	assert.Equal(t, "chunk-b1", got[0].ID)
}

func TestChunkRepository_UpdateMetadataByItemID_overwritesTopicAndStatus_andReturnsUpdatedChunks(t *testing.T) {
	// Given two chunks owned by the same item, and one owned by another
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	a1 := testChunk("chunk-a1", "notes/a.md", now)
	a1.ItemID = "item-a"
	a2 := testChunk("chunk-a2", "notes/a.md", now)
	a2.ItemID = "item-a"
	b1 := testChunk("chunk-b1", "notes/b.md", now)
	b1.ItemID = "item-b"
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{a1, a2, b1}))

	// When updating item-a's metadata
	updated, err := repo.UpdateMetadataByItemID(ctx, "item-a", "Rust", knowledge.StatusDeprecated)

	// Then only item-a's chunks are returned with the new topic/status
	require.NoError(t, err)
	require.Len(t, updated, 2)
	for _, chunk := range updated {
		assert.Equal(t, "Rust", chunk.Topic)
		assert.Equal(t, knowledge.StatusDeprecated, chunk.Status)
	}

	// And the change is persisted, leaving the other item's chunk untouched
	all, listErr := repo.ListAll(ctx)
	require.NoError(t, listErr)
	byID := map[string]knowledge.Chunk{}
	for _, chunk := range all {
		byID[chunk.ID] = chunk
	}
	assert.Equal(t, "Rust", byID["chunk-a1"].Topic)
	assert.Equal(t, knowledge.StatusDeprecated, byID["chunk-a1"].Status)
	assert.Equal(t, "Go", byID["chunk-b1"].Topic)
	assert.Equal(t, knowledge.StatusApproved, byID["chunk-b1"].Status)
}

func TestChunkRepository_UpdateMetadataByItemID_isNoOp_whenNothingMatches(t *testing.T) {
	// Given a repository with no chunks at all
	repo := newTestChunkRepository(t)
	ctx := context.Background()

	// When updating an item ID no chunk carries
	updated, err := repo.UpdateMetadataByItemID(ctx, "item-missing", "Rust", knowledge.StatusDeprecated)

	// Then it succeeds without error and returns no rows
	require.NoError(t, err)
	assert.Empty(t, updated)
}

func TestChunkRepository_ListCurrent_returnsMatchingAthenaChunk_whenItemUpdatedAtMatches(t *testing.T) {
	// Given an athena chunk whose ItemUpdatedAt matches its Item's UpdatedAt
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, items.Save(ctx, testItemAt("item-1", "Go", knowledge.StatusApproved, now)))
	chunk := testChunk("chunk-1", "", now)
	chunk.Source = knowledge.SourceAthena
	chunk.ItemID = "item-1"
	chunk.ItemUpdatedAt = now
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks for the matching embedding model
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is included with no issues
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
	assert.Equal(t, "chunk-1", result.Chunks[0].ID)
	assert.Empty(t, result.Issues)
}

func TestChunkRepository_ListCurrent_returnsImportedDocChunk_regardlessOfItemUpdatedAt(t *testing.T) {
	// Given an imported_doc chunk with no ItemUpdatedAt at all (the normal
	// case — its freshness is governed by ingested_files, not this field)
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusApproved, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now) // testChunk defaults to SourceImportedDoc, zero ItemUpdatedAt
	chunk.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is included despite ItemUpdatedAt never having been set
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
	assert.Empty(t, result.Issues)
}

func TestChunkRepository_ListCurrent_excludesWrongEmbeddingModel_silently(t *testing.T) {
	// Given a valid chunk saved under a different embedding model
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusApproved, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now)
	chunk.ItemID = "item-1"
	chunk.EmbeddingModel = "some-other-model"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks for the expected model
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded from both Chunks and Issues — a different model
	// is expected reindex work, not a corruption warning
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	assert.Empty(t, result.Issues)
}

func TestChunkRepository_ListCurrent_reportsMissingItem_whenNoOwningItemExists(t *testing.T) {
	// Given a chunk whose item_id matches no knowledge_items row
	chunks, _, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	chunk := testChunk("chunk-1", "notes/go.md", now)
	chunk.ItemID = "item-missing"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded and reported with the missing-item reason
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, "chunk-1", result.Issues[0].ChunkID)
	assert.Equal(t, knowledge.ChunkIssueMissingItem, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsSourceMismatch(t *testing.T) {
	// Given a chunk whose Source disagrees with its Item's Source
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusApproved, now) // Source: athena
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now) // Source: imported_doc
	chunk.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded and reported with the source-mismatch reason
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, knowledge.ChunkIssueSourceMismatch, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsTopicMismatch(t *testing.T) {
	// Given a chunk whose Topic disagrees with its Item's Topic
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Rust", knowledge.StatusApproved, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now) // Topic: "Go"
	chunk.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded and reported with the topic-mismatch reason
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, knowledge.ChunkIssueTopicMismatch, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsStatusMismatch(t *testing.T) {
	// Given a chunk whose Status disagrees with its Item's Status
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusDeprecated, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now) // Status: approved
	chunk.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded and reported with the status-mismatch reason
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, knowledge.ChunkIssueStatusMismatch, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsStaleItem_whenAthenaItemUpdatedAtDiffers(t *testing.T) {
	// Given an athena chunk whose ItemUpdatedAt no longer matches its
	// Item's current UpdatedAt (the item changed after this chunk indexed)
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, items.Save(ctx, testItemAt("item-1", "Go", knowledge.StatusApproved, now)))
	chunk := testChunk("chunk-1", "", now)
	chunk.Source = knowledge.SourceAthena
	chunk.ItemID = "item-1"
	chunk.ItemUpdatedAt = now.Add(-time.Hour)
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded and reported with the stale-item reason
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, knowledge.ChunkIssueStaleItem, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsStaleItem_whenAthenaItemUpdatedAtNeverSet(t *testing.T) {
	// Given an athena chunk that never recorded an ItemUpdatedAt at all
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, items.Save(ctx, testItemAt("item-1", "Go", knowledge.StatusApproved, now)))
	chunk := testChunk("chunk-1", "", now)
	chunk.Source = knowledge.SourceAthena
	chunk.ItemID = "item-1"
	// ItemUpdatedAt left at its zero value
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is excluded and reported with the stale-item reason, not
	// silently matched against a mistaken zero-value comparison
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, knowledge.ChunkIssueStaleItem, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsMalformedEmbedding_andStillReturnsOtherValidChunks(t *testing.T) {
	// Given one valid chunk and one whose stored embedding blob is corrupt
	chunks, items, db := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusApproved, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	good := testChunk("chunk-good", "notes/go.md", now)
	good.ItemID = "item-1"
	bad := testChunk("chunk-bad", "notes/go.md", now.Add(time.Second))
	bad.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{good, bad}))
	_, execErr := db.ExecContext(ctx, `UPDATE knowledge_chunks SET embedding = ? WHERE id = ?`, []byte{1, 2, 3}, "chunk-bad")
	require.NoError(t, execErr)

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then the corrupt row is isolated as an issue and the valid one still
	// comes back — one bad chunk never makes the rest unavailable
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
	assert.Equal(t, "chunk-good", result.Chunks[0].ID)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, "chunk-bad", result.Issues[0].ChunkID)
	assert.Equal(t, knowledge.ChunkIssueMalformedEmbedding, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_reportsUnknownSource_asDefenseInDepth_whenItemAgrees(t *testing.T) {
	// Given a chunk and its Item both corrupted to the same unrecognized
	// source value — the mismatch check alone can't catch this, since both
	// sides agree; ValidateChunk is the safety net that still rejects it
	chunks, items, db := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusApproved, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now)
	chunk.ItemID = "item-1"
	// Matches the item's UpdatedAt so the staleness check (which applies to
	// every source except imported_doc, and this chunk is about to stop
	// being one) doesn't mask the unknown-source reason this test targets.
	chunk.ItemUpdatedAt = now
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))
	_, execErr := db.ExecContext(ctx, `UPDATE knowledge_chunks SET source = 'from_the_future' WHERE id = ?`, "chunk-1")
	require.NoError(t, execErr)
	_, execErr = db.ExecContext(ctx, `UPDATE knowledge_items SET source = 'from_the_future' WHERE id = ?`, "item-1")
	require.NoError(t, execErr)

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is still excluded, now with the unknown-source reason
	require.NoError(t, err)
	assert.Empty(t, result.Chunks)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, knowledge.ChunkIssueUnknownSource, result.Issues[0].Reason)
}

func TestChunkRepository_ListCurrent_ordersValidChunksOldestFirst(t *testing.T) {
	// Given two valid imported_doc chunks saved with distinct created_at
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", knowledge.StatusApproved, now)
	item.Source = knowledge.SourceImportedDoc
	require.NoError(t, items.Save(ctx, item))
	older := testChunk("chunk-old", "notes/a.md", now.Add(-time.Hour))
	older.ItemID = "item-1"
	newer := testChunk("chunk-new", "notes/b.md", now)
	newer.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{newer, older}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then they come back oldest first
	require.NoError(t, err)
	require.Len(t, result.Chunks, 2)
	assert.Equal(t, "chunk-old", result.Chunks[0].ID)
	assert.Equal(t, "chunk-new", result.Chunks[1].ID)
}

func TestChunkRepository_ListCurrent_returnsError_onDatabaseWideFailure(t *testing.T) {
	// Given a database that has already been closed
	chunks, _, db := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	require.NoError(t, db.Close())

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then the whole load fails loudly instead of reporting an empty result
	assert.Error(t, err)
	assert.Empty(t, result.Chunks)
}
