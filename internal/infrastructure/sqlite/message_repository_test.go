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

func TestMessageRepository_DeleteBySession_removesEveryMessageForThatSession(t *testing.T) {
	// Given two messages in session-1 and one in session-2
	repo := newTestMessageRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Append(ctx, study.Message{
		ID: "msg-1", SessionID: "session-1", Role: study.RoleUser,
		Content: "Hi", CreatedAt: time.Now().UTC(),
	}))
	require.NoError(t, repo.Append(ctx, study.Message{
		ID: "msg-2", SessionID: "session-1", Role: study.RoleAssistant,
		Content: "Hello!", CreatedAt: time.Now().UTC(),
	}))
	require.NoError(t, repo.Append(ctx, study.Message{
		ID: "msg-3", SessionID: "session-2", Role: study.RoleUser,
		Content: "Still here", CreatedAt: time.Now().UTC(),
	}))

	// When deleting session-1's messages
	err := repo.DeleteBySession(ctx, "session-1")

	// Then only session-2's message remains
	require.NoError(t, err)
	remaining, listErr := repo.ListBySession(ctx, "session-1")
	require.NoError(t, listErr)
	assert.Empty(t, remaining)
	other, listErr := repo.ListBySession(ctx, "session-2")
	require.NoError(t, listErr)
	assert.Len(t, other, 1)
}

func TestMessageRepository_Append_participatesInTransaction(t *testing.T) {
	// Given a repository and a transactor sharing the same *sql.DB
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
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
