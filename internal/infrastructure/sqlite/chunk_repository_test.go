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
	seedSession(t, db, testSessionID)
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
	seedSession(t, db, testSessionID)
	return NewChunkRepository(db), NewKnowledgeRepository(db), db
}

// testItemAt is a matching owner for testChunk: an imported document's Item.
func testItemAt(id, topic string, updatedAt time.Time) knowledge.Item {
	return knowledge.Item{
		ID: id, SessionID: testSessionID, Topic: topic, Concept: "Concept", Definition: "A definition.",
		Source:    knowledge.SourceImportedDoc,
		CreatedAt: updatedAt, UpdatedAt: updatedAt,
	}
}

func testChunk(id, filePath string, createdAt time.Time) knowledge.Chunk {
	return knowledge.Chunk{
		ID:             id,
		SessionID:      testSessionID,
		Source:         knowledge.SourceImportedDoc,
		Topic:          "Go",
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
	// Given a repository and one chunk
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	chunk := testChunk("chunk-1", "notes/go.md", time.Now().UTC().Truncate(time.Second))

	// When saving then listing it
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{chunk}))
	got, err := repo.ListAll(ctx)

	// Then every field round-trips, including the embedding
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, chunk, got[0])
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
	removedIDs, err := repo.DeleteBySourcePath(ctx, testSessionID, "/abs/notes/a.md")

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
	removedIDs, err := repo.DeleteBySourcePath(ctx, testSessionID, "/abs/notes/never-imported.md")

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
	removedIDs, err := repo.DeleteBySourcePath(ctx, testSessionID, "/course-a/notes.md")

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

func TestChunkRepository_ListCurrent_returnsAChunkWhoseItemExists(t *testing.T) {
	// Given an imported_doc chunk whose owning Item exists
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, items.Save(ctx, testItemAt("item-1", "Go", now)))
	chunk := testChunk("chunk-1", "notes/go.md", now)
	chunk.ItemID = "item-1"
	require.NoError(t, chunks.SaveAll(ctx, []knowledge.Chunk{chunk}))

	// When listing current chunks
	result, err := chunks.ListCurrent(ctx, testEmbeddingModel)

	// Then it is included with no issues
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
	assert.Empty(t, result.Issues)
}

func TestChunkRepository_ListCurrent_excludesWrongEmbeddingModel_silently(t *testing.T) {
	// Given a valid chunk saved under a different embedding model
	chunks, items, _ := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", now)
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

func TestChunkRepository_ListCurrent_reportsMalformedEmbedding_andStillReturnsOtherValidChunks(t *testing.T) {
	// Given one valid chunk and one whose stored embedding blob is corrupt
	chunks, items, db := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", now)
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
	// source value; ValidateChunk is the safety net that still rejects it
	chunks, items, db := newTestChunkAndItemRepositories(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	item := testItemAt("item-1", "Go", now)
	require.NoError(t, items.Save(ctx, item))
	chunk := testChunk("chunk-1", "notes/go.md", now)
	chunk.ItemID = "item-1"
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
	item := testItemAt("item-1", "Go", now)
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

func TestChunkRepository_DeleteBySourcePath_leavesTheSameSourcesChunksInOtherSessionsUntouched(t *testing.T) {
	// Given the same source imported into two sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewChunkRepository(db)
	ctx := context.Background()
	inA := testChunk("chunk-a", "notes/go.md", time.Now().UTC())
	inA.SessionID = "session-a"
	inB := testChunk("chunk-b", "notes/go.md", time.Now().UTC())
	inB.SessionID = "session-b"
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{inA, inB}))

	// When re-importing it in session A, which deletes A's previous chunks
	removedIDs, err := repo.DeleteBySourcePath(ctx, "session-a", inA.SourcePath)

	// Then only session A's chunk is removed
	require.NoError(t, err)
	assert.Equal(t, []string{"chunk-a"}, removedIDs)
	remaining, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "chunk-b", remaining[0].ID)
	assert.Equal(t, "session-b", remaining[0].SessionID)
}

func TestChunkRepository_ListIDsBySession_returnsOnlyThatSessionsChunkIDs(t *testing.T) {
	// Given chunks owned by two different sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewChunkRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	a1 := testChunk("chunk-a1", "a1.md", now)
	a1.SessionID = "session-a"
	a2 := testChunk("chunk-a2", "a2.md", now)
	a2.SessionID = "session-a"
	b1 := testChunk("chunk-b1", "b1.md", now)
	b1.SessionID = "session-b"
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{a1, a2, b1}))

	// When listing session A's chunk IDs
	ids, err := repo.ListIDsBySession(ctx, "session-a")

	// Then only A's IDs come back, and nothing is deleted
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"chunk-a1", "chunk-a2"}, ids)
	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestChunkRepository_ListIDsBySession_returnsAnEmptySlice_whenTheSessionOwnsNoChunks(t *testing.T) {
	// Given a session with no chunks
	repo := newTestChunkRepository(t)

	// When listing its chunk IDs
	ids, err := repo.ListIDsBySession(context.Background(), testSessionID)

	// Then the result is empty and non-nil
	require.NoError(t, err)
	assert.NotNil(t, ids)
	assert.Empty(t, ids)
}
