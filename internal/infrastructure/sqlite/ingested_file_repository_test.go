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
	seedSession(t, db, testSessionID)
	return NewIngestedFileRepository(db)
}

func TestIngestedFileRepository_ListBySession_returnsEmptyMap_whenNothingIngestedYet(t *testing.T) {
	// Given a fresh repository
	repo := newTestIngestedFileRepository(t)

	// When listing all ingested files
	files, err := repo.ListBySession(context.Background(), testSessionID)

	// Then it returns an empty, non-nil map
	require.NoError(t, err)
	assert.NotNil(t, files)
	assert.Empty(t, files)
}

func TestIngestedFileRepository_Upsert_thenListBySession_roundTripsEveryField(t *testing.T) {
	// Given a repository and one ingested file record
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	file := knowledge.IngestedFile{
		SessionID:      testSessionID,
		SourcePath:     "/abs/notes/go.md",
		Path:           "notes/go.md",
		MTimeUnixNano:  1700000000123456789,
		EmbeddingModel: "text-embedding-3-small",
		ChunkCount:     3,
		ItemID:         "item-1",
	}

	// When upserting then listing it
	require.NoError(t, repo.Upsert(ctx, file))
	files, err := repo.ListBySession(ctx, testSessionID)

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
		SessionID:  testSessionID,
		SourcePath: "/abs/notes/go.md", Path: "notes/go.md",
		MTimeUnixNano: 1000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-1",
	}))

	// When upserting the same source path again with different values
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID:  testSessionID,
		SourcePath: "/abs/notes/go.md", Path: "notes/go.md",
		MTimeUnixNano: 2000, EmbeddingModel: "model-b", ChunkCount: 5, ItemID: "item-1",
	}))

	// Then there is still exactly one row, holding the latest values
	files, err := repo.ListBySession(ctx, testSessionID)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, int64(2000), files["/abs/notes/go.md"].MTimeUnixNano)
	assert.Equal(t, "model-b", files["/abs/notes/go.md"].EmbeddingModel)
	assert.Equal(t, 5, files["/abs/notes/go.md"].ChunkCount)
}

func TestIngestedFileRepository_ListBySession_keepsTwoSourcesWithTheSameDisplayPath_distinct(t *testing.T) {
	// Given two sources sharing the same display FilePath but reached
	// through different folder roots (e.g. /course-a/notes.md and
	// /course-b/notes.md) — they must coexist, not collide
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID:  testSessionID,
		SourcePath: "/course-a/notes.md", Path: "notes.md",
		MTimeUnixNano: 1000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-a",
	}))
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID:  testSessionID,
		SourcePath: "/course-b/notes.md", Path: "notes.md",
		MTimeUnixNano: 2000, EmbeddingModel: "model-a", ChunkCount: 1, ItemID: "item-b",
	}))

	// When listing all ingested files
	files, err := repo.ListBySession(ctx, testSessionID)

	// Then both are present as distinct records
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "item-a", files["/course-a/notes.md"].ItemID)
	assert.Equal(t, "item-b", files["/course-b/notes.md"].ItemID)
}

func TestIngestedFileRepository_ListBySession_returnsOnlyThatSessionsFiles_andKeepsTheSameSourceInBoth(t *testing.T) {
	// Given the same source imported in two different sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewIngestedFileRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID: "session-a", SourcePath: "/abs/go.md", Path: "go.md",
		MTimeUnixNano: 1, EmbeddingModel: "model", ChunkCount: 1, ItemID: "item-a",
	}))
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID: "session-b", SourcePath: "/abs/go.md", Path: "go.md",
		MTimeUnixNano: 2, EmbeddingModel: "model", ChunkCount: 2, ItemID: "item-b",
	}))

	// When listing each session's files
	filesA, errA := repo.ListBySession(ctx, "session-a")
	filesB, errB := repo.ListBySession(ctx, "session-b")

	// Then each sees only its own record for that source
	require.NoError(t, errA)
	require.NoError(t, errB)
	require.Len(t, filesA, 1)
	require.Len(t, filesB, 1)
	assert.Equal(t, "item-a", filesA["/abs/go.md"].ItemID)
	assert.Equal(t, "session-a", filesA["/abs/go.md"].SessionID)
	assert.Equal(t, "item-b", filesB["/abs/go.md"].ItemID)
}
