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

func TestMessageRepository_ListPage_withoutCursor_returnsTheNewestMessagesFirst(t *testing.T) {
	// Given a session with five messages and another session with one
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1", "msg-2", "msg-3", "msg-4", "msg-5")
	appendTestMessages(t, repo, "session-2", "other-1")

	// When asking for the latest three
	messages, err := repo.ListPage(context.Background(), "session-1", nil, 3)

	// Then they are the three most recent, newest first, of that session only
	require.NoError(t, err)
	assert.Equal(t, []string{"msg-5", "msg-4", "msg-3"}, messageIDs(messages))
}

func TestMessageRepository_ListPage_withACursor_returnsOnlyOlderMessages(t *testing.T) {
	// Given a session with five messages sharing one timestamp
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1", "msg-2", "msg-3", "msg-4", "msg-5")
	cursor, err := study.NewMessageCursor("msg-4")
	require.NoError(t, err)

	// When asking for the page before msg-4
	messages, err := repo.ListPage(context.Background(), "session-1", &cursor, 10)

	// Then only the older messages come back, newest first, cursor excluded
	require.NoError(t, err)
	assert.Equal(t, []string{"msg-3", "msg-2", "msg-1"}, messageIDs(messages))
}

func TestMessageRepository_ListPage_orderedByTimestampBeforeInsertionOrder(t *testing.T) {
	// Given a message inserted last but stamped earlier than the others
	repo := newTestMessageRepository(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second)
	for i, id := range []string{"msg-late", "msg-early"} {
		offsets := []time.Duration{time.Minute, 0}
		require.NoError(t, repo.Append(ctx, study.Message{
			ID: id, SessionID: "session-1", Role: study.RoleUser, Content: id, CreatedAt: base.Add(offsets[i]),
		}))
	}

	// When asking for the latest page
	messages, err := repo.ListPage(ctx, "session-1", nil, 10)

	// Then the timestamp wins over insertion order
	require.NoError(t, err)
	assert.Equal(t, []string{"msg-late", "msg-early"}, messageIDs(messages))
}

func TestMessageRepository_ListPage_walkingTheCursorsVisitsEveryMessageExactlyOnce(t *testing.T) {
	// Given a session whose messages share a timestamp and whose count is an
	// exact multiple of the page size
	repo := newTestMessageRepository(t)
	ctx := context.Background()
	want := []string{"msg-01", "msg-02", "msg-03", "msg-04", "msg-05", "msg-06"}
	appendTestMessages(t, repo, "session-1", want...)
	const pageSize = 3

	// When following nextCursor from the latest page until the history ends
	var got []string
	var cursor *study.MessageCursor
	for {
		newestFirst, err := repo.ListPage(ctx, "session-1", cursor, pageSize+1)
		require.NoError(t, err)
		page := study.NewMessagePage(newestFirst, pageSize)
		got = append(messageIDs(page.Messages), got...)
		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
	}

	// Then every message appears once, in conversation order
	assert.Equal(t, want, got)
}

func TestMessageRepository_ListPage_returnsEmptyForAnEmptySession(t *testing.T) {
	// Given a repository with no messages
	repo := newTestMessageRepository(t)

	// When asking for the latest page of a session with none
	messages, err := repo.ListPage(context.Background(), "session-1", nil, 10)

	// Then it returns an empty slice, not an error
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestMessageRepository_ListPage_cursorAtTheOldestMessageReturnsAnEmptyPage(t *testing.T) {
	// Given a session and a cursor on its oldest message
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1", "msg-2")
	cursor, err := study.NewMessageCursor("msg-1")
	require.NoError(t, err)

	// When asking for the page before it
	messages, err := repo.ListPage(context.Background(), "session-1", &cursor, 10)

	// Then there is nothing older, and that is not an error
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestMessageRepository_ListPage_rejectsACursorThatIsNotInTheSession(t *testing.T) {
	// Given a message in another session and an id that exists nowhere
	repo := newTestMessageRepository(t)
	appendTestMessages(t, repo, "session-1", "msg-1")
	appendTestMessages(t, repo, "session-2", "other-1", "other-2")

	for _, id := range []string{"other-2", "missing"} {
		cursor, err := study.NewMessageCursor(id)
		require.NoError(t, err)

		// When paging session-1 with that cursor
		messages, err := repo.ListPage(context.Background(), "session-1", &cursor, 10)

		// Then it fails and returns no data
		require.ErrorIs(t, err, study.ErrMessageNotInSession, id)
		assert.Empty(t, messages, id)
	}
}
