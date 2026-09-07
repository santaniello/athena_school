package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// setupMessageSourceFixture relies on the "default" folder migrations
// already seed on every Open — it never inserts one itself.
func setupMessageSourceFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES ('session-1', 'Go', 'socratic', 'default', CURRENT_TIMESTAMP);
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES
			('message-1', 'session-1', 'assistant', 'Goroutines are cheap.', CURRENT_TIMESTAMP),
			('message-2', 'session-1', 'assistant', 'Channels connect goroutines.', CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
}

func TestMessageSourceRepository_SaveThenListBySession_returnsSourcesInOrderPerMessage(t *testing.T) {
	// Given two messages in the same session, each with its own ordered sources
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	setupMessageSourceFixture(t, db)

	repo := NewMessageSourceRepository(db)
	firstMessageSources := []domainknowledge.Source{
		{ChunkID: "chunk-1", ItemID: "item-1", SourceType: "athena", Concept: "Goroutines", Score: 0.9, Excerpt: "cheap"},
		{ChunkID: "chunk-2", ItemID: "item-2", SourceType: "athena", Concept: "Scheduler", Score: 0.6, Excerpt: "M:N"},
	}
	secondMessageSources := []domainknowledge.Source{
		{ChunkID: "chunk-3", ItemID: "item-3", SourceType: "user_note", FilePath: "notes.md", Concept: "Channels", Score: 0.7, Excerpt: "typed"},
	}

	// When saving both messages' sources
	require.NoError(t, repo.Save(context.Background(), "message-1", firstMessageSources))
	require.NoError(t, repo.Save(context.Background(), "message-2", secondMessageSources))

	// Then listing the session returns each message's sources, in order
	result, err := repo.ListBySession(context.Background(), "session-1")
	require.NoError(t, err)
	require.Len(t, result["message-1"], 2)
	assert.Equal(t, "chunk-1", result["message-1"][0].ChunkID)
	assert.Equal(t, "chunk-2", result["message-1"][1].ChunkID)
	require.Len(t, result["message-2"], 1)
	assert.Equal(t, "chunk-3", result["message-2"][0].ChunkID)
	assert.Equal(t, "notes.md", result["message-2"][0].FilePath)
}

func TestMessageSourceRepository_ListBySession_omitsMessagesWithNoPersistedSources(t *testing.T) {
	// Given a message that never had sources saved for it
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	setupMessageSourceFixture(t, db)
	repo := NewMessageSourceRepository(db)

	// When listing the session
	result, err := repo.ListBySession(context.Background(), "session-1")
	require.NoError(t, err)

	// Then the message is simply absent from the map
	_, present := result["message-1"]
	assert.False(t, present)
}

func TestMessageSourceRepository_Save_withEmptySlice_isANoOp(t *testing.T) {
	// Given a message
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	setupMessageSourceFixture(t, db)
	repo := NewMessageSourceRepository(db)

	// When saving an empty source list
	err = repo.Save(context.Background(), "message-1", nil)

	// Then it succeeds without persisting anything
	require.NoError(t, err)
	result, listErr := repo.ListBySession(context.Background(), "session-1")
	require.NoError(t, listErr)
	assert.Empty(t, result)
}

func TestMessageSourceRepository_Save_replacesAnyPreviouslySavedSourcesForTheSameMessage(t *testing.T) {
	// Given a message with sources already saved
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	setupMessageSourceFixture(t, db)
	repo := NewMessageSourceRepository(db)
	require.NoError(t, repo.Save(context.Background(), "message-1", []domainknowledge.Source{
		{ChunkID: "stale-chunk", ItemID: "item-1", SourceType: "athena", Score: 0.5},
	}))

	// When saving again with different sources
	require.NoError(t, repo.Save(context.Background(), "message-1", []domainknowledge.Source{
		{ChunkID: "fresh-chunk", ItemID: "item-1", SourceType: "athena", Score: 0.8},
	}))

	// Then only the latest save survives
	result, err := repo.ListBySession(context.Background(), "session-1")
	require.NoError(t, err)
	require.Len(t, result["message-1"], 1)
	assert.Equal(t, "fresh-chunk", result["message-1"][0].ChunkID)
}
