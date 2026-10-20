package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/study"
)

func newTestMessageRepository(t *testing.T) *MessageRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES
		('session-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP),
		('session-2', 'Rust', 'socratic', 'folder-1', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return NewMessageRepository(db)
}

func TestMessageRepository_Append_thenListBySession_returnsInOrder(t *testing.T) {
	// Given a repository and two messages appended in order
	repo := newTestMessageRepository(t)
	ctx := context.Background()
	first := study.Message{
		ID: "msg-1", SessionID: "session-1", Role: study.RoleUser,
		Content: "What is CAP theorem?", CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	second := study.Message{
		ID: "msg-2", SessionID: "session-1", Role: study.RoleAssistant,
		Content: "It stands for...", CreatedAt: first.CreatedAt.Add(time.Second),
	}
	require.NoError(t, repo.Append(ctx, first))
	require.NoError(t, repo.Append(ctx, second))

	// When listing messages for the session
	messages, err := repo.ListBySession(ctx, "session-1")

	// Then both are returned in the order they were sent
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, first.Content, messages[0].Content)
	assert.Equal(t, first.Role, messages[0].Role)
	assert.Equal(t, second.Content, messages[1].Content)
	assert.Equal(t, second.Role, messages[1].Role)
}

func TestMessageRepository_ListBySession_returnsEmptyForUnknownSession(t *testing.T) {
	// Given a repository with no messages
	repo := newTestMessageRepository(t)
	ctx := context.Background()

	// When listing messages for a session that has none
	messages, err := repo.ListBySession(ctx, "unknown-session")

	// Then it returns an empty slice, not an error
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func appendTestMessages(t *testing.T, repo *MessageRepository, sessionID string, ids ...string) {
	t.Helper()
	// All messages share one timestamp so ordering must fall back to insertion order.
	createdAt := time.Now().UTC().Truncate(time.Second)
	for _, id := range ids {
		require.NoError(t, repo.Append(context.Background(), study.Message{
			ID: id, SessionID: sessionID, Role: study.RoleUser, Content: id, CreatedAt: createdAt,
		}))
	}
}

func messageIDs(messages []study.Message) []string {
	ids := make([]string, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.ID)
	}
	return ids
}

func TestMessageRepository_ListAfter_emptyCursorReturnsEverything(t *testing.T) {
	// Given a session with three messages
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1", "msg-2", "msg-3")

	// When listing after an empty cursor
	messages, err := repo.ListAfter(context.Background(), "session-1", "")

	// Then every message is returned in insertion order
	require.NoError(t, err)
	assert.Equal(t, []string{"msg-1", "msg-2", "msg-3"}, messageIDs(messages))
}

func TestMessageRepository_ListAfter_returnsOnlyMessagesAfterTheAnchor(t *testing.T) {
	// Given a session with three messages and another session with one
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1", "msg-2", "msg-3")
	appendTestMessages(t, repo, "session-2", "other-1")

	// When listing after the first message
	messages, err := repo.ListAfter(context.Background(), "session-1", "msg-1")

	// Then only the later messages of that session are returned
	require.NoError(t, err)
	assert.Equal(t, []string{"msg-2", "msg-3"}, messageIDs(messages))
}

func TestMessageRepository_ListAfter_lastMessageAnchorReturnsEmpty(t *testing.T) {
	// Given a session whose last message is the anchor
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1", "msg-2")

	// When listing after the last message
	messages, err := repo.ListAfter(context.Background(), "session-1", "msg-2")

	// Then it returns an empty slice, not an error
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestMessageRepository_ListAfter_anchorFromAnotherSessionIsRejected(t *testing.T) {
	// Given an anchor that belongs to a different session
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1")
	appendTestMessages(t, repo, "session-2", "other-1")

	// When listing session-1 after that anchor
	messages, err := repo.ListAfter(context.Background(), "session-1", "other-1")

	// Then it fails with ErrMessageNotInSession and returns nothing
	require.ErrorIs(t, err, study.ErrMessageNotInSession)
	assert.Nil(t, messages)
}

func TestMessageRepository_ListAfter_unknownAnchorIsRejected(t *testing.T) {
	// Given a session and an anchor that does not exist
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1")

	// When listing after the unknown anchor
	_, err := repo.ListAfter(context.Background(), "session-1", "ghost")

	// Then it fails with ErrMessageNotInSession
	require.ErrorIs(t, err, study.ErrMessageNotInSession)
}

func TestMessageRepository_Append_participatesInTransaction(t *testing.T) {
	// Given a repository and a transactor sharing the same *sql.DB
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, topic, mode, folder_id, started_at)
		VALUES ('session-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	repo := NewMessageRepository(db)
	transactor := NewSQLTransactor(db)
	ctx := context.Background()

	// When an Append inside a failed transaction is rolled back
	txErr := transactor.WithinTx(ctx, func(ctx context.Context) error {
		if err := repo.Append(ctx, study.Message{
			ID: "msg-1", SessionID: "session-1", Role: study.RoleUser, Content: "Hi", CreatedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		return assert.AnError
	})

	// Then the transaction fails and the message never landed
	require.Error(t, txErr)
	messages, listErr := repo.ListBySession(ctx, "session-1")
	require.NoError(t, listErr)
	assert.Empty(t, messages)
}
