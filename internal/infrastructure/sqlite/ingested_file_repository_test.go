package sqlite

import (
	"context"
	"database/sql"
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

// insertSessionSourceRow writes one knowledge_items row and one matching
// ingested_files row directly (bypassing Upsert, whose ingested_at is
// always CURRENT_TIMESTAMP) so ordering tests can control ingestedAt.
func insertSessionSourceRow(t *testing.T, db *sql.DB, sessionID, itemID, concept, filePath string, chunkCount int, ingestedAt string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO knowledge_items
		(id, session_id, topic, concept, definition, properties, trade_offs, related_concepts, source, created_at, updated_at)
		VALUES (?, ?, 'Go', ?, 'A document.', '[]', '[]', '[]', 'imported_doc', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		itemID, sessionID, concept,
	)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO ingested_files
		(session_id, source_path, file_path, mtime_unix_nano, embedding_model, chunk_count, item_id, ingested_at)
		VALUES (?, ?, ?, 1, 'model', ?, ?, ?)`,
		sessionID, "/abs/"+filePath, filePath, chunkCount, itemID, ingestedAt,
	)
	require.NoError(t, err)
}

func TestIngestedFileRepository_ListSourcesBySession_returnsEmpty_whenNothingIngestedYet(t *testing.T) {
	// Given a fresh repository
	repo := newTestIngestedFileRepository(t)

	// When listing the session's sources
	sources, err := repo.ListSourcesBySession(context.Background(), testSessionID)

	// Then it returns an empty, non-nil slice
	require.NoError(t, err)
	assert.NotNil(t, sources)
	assert.Empty(t, sources)
}

func TestIngestedFileRepository_ListSourcesBySession_joinsTitleFromItem_orderedOldestFirst(t *testing.T) {
	// Given two imported documents, ingested in reverse chronological
	// insertion order
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, testSessionID)
	repo := NewIngestedFileRepository(db)
	insertSessionSourceRow(t, db, testSessionID, "item-newer", "CAP theorem", "cap.md", 4, "2024-01-02 00:00:00")
	insertSessionSourceRow(t, db, testSessionID, "item-older", "Distributed Systems", "notes/ds.md", 12, "2024-01-01 00:00:00")

	// When listing the session's sources
	sources, listErr := repo.ListSourcesBySession(context.Background(), testSessionID)

	// Then they come back oldest-imported first, with the item's concept
	// as Title and the display FilePath as Path
	require.NoError(t, listErr)
	require.Len(t, sources, 2)
	assert.Equal(t, knowledge.SessionSource{
		ItemID: "item-older", Title: "Distributed Systems", Path: "notes/ds.md",
		ChunkCount: 12, IngestedAt: sources[0].IngestedAt,
	}, sources[0])
	assert.Equal(t, "item-newer", sources[1].ItemID)
	assert.True(t, sources[0].IngestedAt.Before(sources[1].IngestedAt))
}

func TestIngestedFileRepository_ListSourcesBySession_excludesRecordWhoseItemWasDeleted(t *testing.T) {
	// Given an ingested_files row whose knowledge_items row is gone (the
	// item was deleted without going through Remove's own cleanup, or the
	// two writes happened in separate transactions)
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID: testSessionID, SourcePath: "/abs/orphan.md", Path: "orphan.md",
		MTimeUnixNano: 1, EmbeddingModel: "model", ChunkCount: 1, ItemID: "missing-item",
	}))

	// When listing the session's sources
	sources, err := repo.ListSourcesBySession(ctx, testSessionID)

	// Then the orphaned record is not listed
	require.NoError(t, err)
	assert.Empty(t, sources)
}

func TestIngestedFileRepository_ListSourcesBySession_returnsOnlyThatSessionsSources(t *testing.T) {
	// Given the same source imported into two different sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewIngestedFileRepository(db)
	insertSessionSourceRow(t, db, "session-a", "item-a", "Go", "go.md", 1, "2024-01-01 00:00:00")
	insertSessionSourceRow(t, db, "session-b", "item-b", "Go", "go.md", 2, "2024-01-01 00:00:00")

	// When listing session-a's sources
	sources, listErr := repo.ListSourcesBySession(context.Background(), "session-a")

	// Then only session-a's document is returned
	require.NoError(t, listErr)
	require.Len(t, sources, 1)
	assert.Equal(t, "item-a", sources[0].ItemID)
}

func TestIngestedFileRepository_DeleteByItemID_removesTheMatchingRecord(t *testing.T) {
	// Given an ingested file record
	repo := newTestIngestedFileRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID: testSessionID, SourcePath: "/abs/go.md", Path: "go.md",
		MTimeUnixNano: 1, EmbeddingModel: "model", ChunkCount: 1, ItemID: "item-1",
	}))

	// When deleting it by its item id
	err := repo.DeleteByItemID(ctx, testSessionID, "item-1")

	// Then it is gone
	require.NoError(t, err)
	files, listErr := repo.ListBySession(ctx, testSessionID)
	require.NoError(t, listErr)
	assert.Empty(t, files)
}

func TestIngestedFileRepository_DeleteByItemID_neverTouchesAnotherSessionsRecord(t *testing.T) {
	// Given the same source imported into two sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewIngestedFileRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID: "session-a", SourcePath: "/abs/go.md", Path: "go.md",
		MTimeUnixNano: 1, EmbeddingModel: "model", ChunkCount: 1, ItemID: "item-1",
	}))
	require.NoError(t, repo.Upsert(ctx, knowledge.IngestedFile{
		SessionID: "session-b", SourcePath: "/abs/go.md", Path: "go.md",
		MTimeUnixNano: 2, EmbeddingModel: "model", ChunkCount: 2, ItemID: "item-1",
	}))

	// When deleting session-a's record by item id
	deleteErr := repo.DeleteByItemID(ctx, "session-a", "item-1")

	// Then session-b's record for the same item id is untouched
	require.NoError(t, deleteErr)
	filesB, listErr := repo.ListBySession(ctx, "session-b")
	require.NoError(t, listErr)
	assert.Len(t, filesB, 1)
}

func TestIngestedFileRepository_DeleteByItemID_noMatch_isNoop(t *testing.T) {
	// Given a repository with no matching record
	repo := newTestIngestedFileRepository(t)

	// When deleting a non-existent item id
	err := repo.DeleteByItemID(context.Background(), testSessionID, "no-such-item")

	// Then it succeeds without error
	require.NoError(t, err)
}
