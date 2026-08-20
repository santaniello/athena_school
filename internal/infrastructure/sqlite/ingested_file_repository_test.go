package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

func newTestIngestedFileRepository(t *testing.T) *IngestedFileRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewIngestedFileRepository(db)
}

func TestIngestedFileRepository_ListAll_returnsEmptyMap_whenNothingIngestedYet(t *testing.T) {
	// Given a fresh repository
	repo := newTestIngestedFileRepository(t)

	// When listing all ingested files
	files, err := repo.ListAll(context.Background())

	// Then it returns an empty, non-nil map
	require.NoError(t, err)
	assert.NotNil(t, files)
	assert.Empty(t, files)
}

func TestIngestedFileRepository_Upsert_thenListAll_roundTripsEveryField(t *testing.T) {
	// Given a repository and one ingested file record
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	file := knowledge.IngestedFile{
		SourcePath:     "/abs/notes/go.md",
		Path:           "notes/go.md",
		MTimeUnixNano:  1700000000123456789,
		EmbeddingModel: "text-embedding-3-small",
		ChunkCount:     3,
		ItemID:         "item-1",
	}

	// When upserting then listing it
	require.NoError(t, repo.Upsert(ctx, file))
	files, err := repo.ListAll(ctx)

	// Then it round-trips, keyed by SourcePath
	require.NoError(t, err)
	require.Contains(t, files, "/abs/notes/go.md")
	assert.Equal(t, file, files["/abs/notes/go.md"])
}

func TestIngestedFileRepository_Upsert_replacesExistingRow_forSameSourcePath(t *testing.T) {
	// Given an already-ingested file
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SourcePath: "/abs/notes/go.md", Path: "notes/go.md",
		MTimeUnixNano: 1000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-1",
	}))

	// When upserting the same source path again with different values
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SourcePath: "/abs/notes/go.md", Path: "notes/go.md",
		MTimeUnixNano: 2000, EmbeddingModel: "model-b", ChunkCount: 5, ItemID: "item-1",
	}))

	// Then there is still exactly one row, holding the latest values
	files, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, int64(2000), files["/abs/notes/go.md"].MTimeUnixNano)
	assert.Equal(t, "model-b", files["/abs/notes/go.md"].EmbeddingModel)
	assert.Equal(t, 5, files["/abs/notes/go.md"].ChunkCount)
}

func TestIngestedFileRepository_ListAll_keepsTwoSourcesWithTheSameDisplayPath_distinct(t *testing.T) {
	// Given two sources sharing the same display FilePath but reached
	// through different folder roots (e.g. /course-a/notes.md and
	// /course-b/notes.md) — they must coexist, not collide
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SourcePath: "/course-a/notes.md", Path: "notes.md",
		MTimeUnixNano: 1000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-a",
	}))
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SourcePath: "/course-b/notes.md", Path: "notes.md",
		MTimeUnixNano: 2000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-b",
	}))

	// When listing all ingested files
	files, err := repo.ListAll(ctx)

	// Then both are present as distinct records
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "item-a", files["/course-a/notes.md"].ItemID)
	assert.Equal(t, "item-b", files["/course-b/notes.md"].ItemID)
}
