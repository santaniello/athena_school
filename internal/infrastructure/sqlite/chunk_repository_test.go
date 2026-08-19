package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

func newTestChunkRepository(t *testing.T) *ChunkRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewChunkRepository(db)
}

func testChunk(id, filePath string, createdAt time.Time) knowledge.Chunk {
	return knowledge.Chunk{
		ID:             id,
		Source:         knowledge.SourceImportedDoc,
		Topic:          "Go",
		Status:         knowledge.StatusApproved,
		ItemID:         "item-" + id,
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

func TestChunkRepository_DeleteByFilePath_removesOnlyThatFilesChunks(t *testing.T) {
	// Given chunks from two different files
	repo := newTestChunkRepository(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.SaveAll(ctx, []knowledge.Chunk{
		testChunk("chunk-a1", "notes/a.md", now),
		testChunk("chunk-a2", "notes/a.md", now),
		testChunk("chunk-b1", "notes/b.md", now),
	}))

	// When deleting by one file's path
	err := repo.DeleteByFilePath(ctx, "notes/a.md")

	// Then only that file's chunks are gone
	require.NoError(t, err)
	got, listErr := repo.ListAll(ctx)
	require.NoError(t, listErr)
	require.Len(t, got, 1)
	assert.Equal(t, "chunk-b1", got[0].ID)
}

func TestChunkRepository_DeleteByFilePath_isNoOp_whenNothingMatches(t *testing.T) {
	// Given a repository with no chunks at all
	repo := newTestChunkRepository(t)
	ctx := context.Background()

	// When deleting by a file path that was never ingested
	err := repo.DeleteByFilePath(ctx, "notes/never-imported.md")

	// Then it succeeds without error
	assert.NoError(t, err)
}

func TestChunkRepository_DeleteByItemID_removesOnlyThatItemsChunks(t *testing.T) {
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
	err := repo.DeleteByItemID(ctx, "item-a")

	// Then only that item's chunks are gone
	require.NoError(t, err)
	got, listErr := repo.ListAll(ctx)
	require.NoError(t, listErr)
	require.Len(t, got, 1)
	assert.Equal(t, "chunk-b1", got[0].ID)
}
