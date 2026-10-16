package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

func setupConversationSummaryFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP);
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES
			('session-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP),
			('session-2', 'Rust', 'socratic', 'folder-1', CURRENT_TIMESTAMP);
		INSERT INTO messages (id, session_id, role, content, created_at) VALUES
			('message-1', 'session-1', 'user', 'What is a goroutine?', CURRENT_TIMESTAMP),
			('message-2', 'session-1', 'assistant', 'A lightweight thread.', CURRENT_TIMESTAMP),
			('message-3', 'session-2', 'user', 'What is ownership?', CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
}

func newSummary(t *testing.T, sessionID, content, coversUntil string) domainstudy.ConversationSummary {
	t.Helper()
	summary, err := domainstudy.NewConversationSummary(
		sessionID, content, coversUntil, 10, time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	return summary
}

func TestConversationSummaryRepository_SaveThenGet_returnsSummary(t *testing.T) {
	// Given
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)
	want := newSummary(t, "session-1", "## Progress\n- goroutines", "message-2")

	// When
	require.NoError(t, repo.Save(context.Background(), want))
	got, err := repo.Get(context.Background(), "session-1")

	// Then
	require.NoError(t, err)
	assert.Equal(t, want.SessionID, got.SessionID)
	assert.Equal(t, want.Content, got.Content)
	assert.Equal(t, want.CoversUntilMessageID, got.CoversUntilMessageID)
	assert.Equal(t, want.TokenEstimate, got.TokenEstimate)
	assert.True(t, want.UpdatedAt.Equal(got.UpdatedAt), "updated_at %v != %v", want.UpdatedAt, got.UpdatedAt)
}

func TestConversationSummaryRepository_Save_replacesExistingSummary(t *testing.T) {
	// Given a saved summary
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)
	require.NoError(t, repo.Save(context.Background(), newSummary(t, "session-1", "old", "message-1")))

	// When saving a new one for the same session
	require.NoError(t, repo.Save(context.Background(), newSummary(t, "session-1", "new", "message-2")))

	// Then only the new one is kept
	got, err := repo.Get(context.Background(), "session-1")
	require.NoError(t, err)
	assert.Equal(t, "new", got.Content)
	assert.Equal(t, "message-2", got.CoversUntilMessageID)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM conversation_summaries`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestConversationSummaryRepository_Get_returnsNotFound_whenSessionHasNoSummary(t *testing.T) {
	// Given
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)

	// When
	_, err := repo.Get(context.Background(), "session-1")

	// Then
	assert.ErrorIs(t, err, domainstudy.ErrConversationSummaryNotFound)
}

func TestConversationSummaryRepository_Save_failsAndPersistsNothing_whenMessageBelongsToAnotherSession(t *testing.T) {
	// Given message-3 belongs to session-2
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)

	// When saving a session-1 summary that covers it
	err := repo.Save(context.Background(), newSummary(t, "session-1", "x", "message-3"))

	// Then
	assert.ErrorIs(t, err, domainstudy.ErrSummaryMessageNotInSession)
	_, getErr := repo.Get(context.Background(), "session-1")
	assert.ErrorIs(t, getErr, domainstudy.ErrConversationSummaryNotFound)
}

func TestConversationSummaryRepository_Save_keepsPreviousSummary_whenReplacementIsRejected(t *testing.T) {
	// Given a valid saved summary
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)
	require.NoError(t, repo.Save(context.Background(), newSummary(t, "session-1", "kept", "message-1")))

	// When a replacement pointing at a foreign message is rejected
	err := repo.Save(context.Background(), newSummary(t, "session-1", "bad", "message-3"))

	// Then the previous summary is intact
	require.ErrorIs(t, err, domainstudy.ErrSummaryMessageNotInSession)
	got, getErr := repo.Get(context.Background(), "session-1")
	require.NoError(t, getErr)
	assert.Equal(t, "kept", got.Content)
}

func TestConversationSummaryRepository_Save_failsForUnknownMessage(t *testing.T) {
	// Given
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)

	// When
	err := repo.Save(context.Background(), newSummary(t, "session-1", "x", "missing"))

	// Then
	assert.ErrorIs(t, err, domainstudy.ErrSummaryMessageNotInSession)
}

func TestConversationSummaryRepository_summaryIsRemoved_whenSessionIsDeleted(t *testing.T) {
	// Given
	db := newTestDB(t)
	setupConversationSummaryFixture(t, db)
	repo := NewConversationSummaryRepository(db)
	require.NoError(t, repo.Save(context.Background(), newSummary(t, "session-1", "x", "message-2")))

	// When the session is deleted
	_, err := db.Exec(`DELETE FROM sessions WHERE id = 'session-1'`)
	require.NoError(t, err)

	// Then the summary cascades away
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM conversation_summaries`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestOpen_createsConversationSummariesTable_onADatabaseThatPredatesIt(t *testing.T) {
	// Given a database opened before the conversation_summaries migration existed
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	_, err = first.Exec(`DROP TABLE conversation_summaries`)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the table exists again
	var tableName string
	require.NoError(t, db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'conversation_summaries'`,
	).Scan(&tableName))
}
