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
		Path:           "notes/go.md",
		MTime:          1700000000,
		EmbeddingModel: "text-embedding-3-small",
		ChunkCount:     3,
		ItemID:         "item-1",
	}

	// When upserting then listing it
	require.NoError(t, repo.Upsert(ctx, file))
	files, err := repo.ListAll(ctx)

	// Then it round-trips, keyed by path
	require.NoError(t, err)
	require.Contains(t, files, "notes/go.md")
	assert.Equal(t, file, files["notes/go.md"])
}

func TestIngestedFileRepository_Upsert_replacesExistingRow_forSamePath(t *testing.T) {
	// Given an already-ingested file
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		Path: "notes/go.md", MTime: 1000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-1",
	}))

	// When upserting the same path again with different values
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		Path: "notes/go.md", MTime: 2000, EmbeddingModel: "model-b", ChunkCount: 5, ItemID: "item-1",
	}))

	// Then there is still exactly one row, holding the latest values
	files, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, int64(2000), files["notes/go.md"].MTime)
	assert.Equal(t, "model-b", files["notes/go.md"].EmbeddingModel)
	assert.Equal(t, 5, files["notes/go.md"].ChunkCount)
}
