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

func newTestSessionRepository(t *testing.T) *SessionRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewSessionRepository(db)
}

// normalContext is the default, unmeasured ContextUsage every freshly
// constructed session in these tests uses unless a test cares about a
// different value — mirrors what application/study.Start sets for real.
var normalContext = study.ContextUsage{State: study.ContextStateNormal}

func TestSessionRepository_Create_storesSession(t *testing.T) {
	// Given a repository and a new study session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{
		ID:        "session-1",
		Topic:     "Distributed systems",
		Mode:      study.ModeStudy,
		FolderID:  "default",
		StartedAt: time.Now().UTC().Truncate(time.Second),
		Context:   normalContext,
	}

	// When creating it
	err := repo.Create(ctx, session)

	// Then it succeeds and the row is queryable
	require.NoError(t, err)
	var topic, mode, folderID string
	queryErr := repo.db.QueryRowContext(ctx,
		`SELECT topic, mode, folder_id FROM sessions WHERE id = ?`, session.ID,
	).Scan(&topic, &mode, &folderID)
	require.NoError(t, queryErr)
	assert.Equal(t, session.Topic, topic)
	assert.Equal(t, session.Mode, mode)
	assert.Equal(t, session.FolderID, folderID)
}

func TestSessionRepository_Create_storesContextUsage(t *testing.T) {
	// Given a repository and a session with non-default context usage
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(),
		Context: study.ContextUsage{
			State: study.ContextStateWarning, Model: "anthropic/claude-sonnet-4.5",
			UsedTokens: 12345, ContextLength: 200000, Estimated: true,
		},
	}

	// When creating it
	err := repo.Create(ctx, session)

	// Then the stored context round-trips exactly
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, session.ID)
	require.NoError(t, getErr)
	assert.Equal(t, session.Context, stored.Context)
}

func TestSessionRepository_Create_rejectsInvalidContextState(t *testing.T) {
	// Given a session with an unrecognized context state
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(),
		Context: study.ContextUsage{State: "bogus"},
	}

	// When creating it
	err := repo.Create(ctx, session)

	// Then it is rejected before touching SQL, not silently normalized
	require.Error(t, err)
}

func TestSessionRepository_Create_rejectsNegativeContextTokens(t *testing.T) {
	// Given a session with a negative used-tokens count
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(),
		Context: study.ContextUsage{State: study.ContextStateNormal, UsedTokens: -1},
	}

	// When creating it
	err := repo.Create(ctx, session)

	// Then it is rejected
	require.Error(t, err)
}

func TestSessionRepository_GetByID_returnsStoredSession(t *testing.T) {
	// Given a repository with an existing session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{
		ID: "session-1", Topic: "Topic", Mode: study.ModeStudy, FolderID: "default",
		StartedAt: time.Now().UTC().Truncate(time.Second), Context: normalContext,
	}
	require.NoError(t, repo.Create(ctx, session))

	// When fetching it by id
	stored, err := repo.GetByID(ctx, "session-1")

	// Then the stored fields are returned, including the default/unmeasured
	// context usage
	require.NoError(t, err)
	assert.Equal(t, session.Topic, stored.Topic)
	assert.Equal(t, session.FolderID, stored.FolderID)
	assert.Equal(t, normalContext, stored.Context)
}

func TestSessionRepository_GetByID_returnsNotFound_whenSessionDoesNotExist(t *testing.T) {
	// Given a repository with no sessions
	repo := newTestSessionRepository(t)
	ctx := context.Background()

	// When fetching a session that does not exist
	_, err := repo.GetByID(ctx, "missing-session")

	// Then it fails with ErrSessionNotFound
	assert.ErrorIs(t, err, study.ErrSessionNotFound)
}

func TestSessionRepository_GetByID_returnsDecodeError_forUnknownPersistedContextState(t *testing.T) {
	// Given a session row whose context_state was corrupted/predates the
	// enum (bypassing repository validation via a raw SQL write, simulating
	// data written by a future/different version)
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(), Context: normalContext,
	}))
	_, err := repo.db.ExecContext(ctx, `UPDATE sessions SET context_state = 'bogus' WHERE id = ?`, "session-1")
	require.NoError(t, err)

	// When fetching it
	_, getErr := repo.GetByID(ctx, "session-1")

	// Then it is a decode error, not silently normalized
	require.Error(t, getErr)
}

func TestSessionRepository_ListByFolder_returnsOnlySessionsInThatFolder(t *testing.T) {
	// Given sessions spread across two folders
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-2", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-3", Mode: study.ModeStudy, FolderID: "folder-b", StartedAt: time.Now().UTC(), Context: normalContext}))

	// When listing sessions in folder-a
	sessions, err := repo.ListByFolder(ctx, "folder-a")

	// Then only its two sessions are returned
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestSessionRepository_ListByFolder_failsEntirely_whenAnyRowIsMalformed(t *testing.T) {
	// Given one well-formed session and one with a corrupted context state
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-2", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}))
	_, err := repo.db.ExecContext(ctx, `UPDATE sessions SET context_state = 'bogus' WHERE id = ?`, "s-2")
	require.NoError(t, err)

	// When listing the folder
	_, listErr := repo.ListByFolder(ctx, "folder-a")

	// Then the whole listing fails rather than silently omitting the bad row
	require.Error(t, listErr)
}

func TestSessionRepository_MoveToFolder_updatesFolderID(t *testing.T) {
	// Given a repository with a session in one folder
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{ID: "session-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}
	require.NoError(t, repo.Create(ctx, session))

	// When moving it to another folder
	err := repo.MoveToFolder(ctx, session.ID, "folder-b")

	// Then it is now in the new folder
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, session.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "folder-b", stored.FolderID)
}

func TestSessionRepository_MoveToFolder_returnsNotFound_whenSessionDoesNotExist(t *testing.T) {
	// Given a repository with no sessions
	repo := newTestSessionRepository(t)
	ctx := context.Background()

	// When moving a session that does not exist
	err := repo.MoveToFolder(ctx, "missing-session", "folder-b")

	// Then it fails with ErrSessionNotFound
	assert.ErrorIs(t, err, study.ErrSessionNotFound)
}

func TestSessionRepository_Delete_removesSession(t *testing.T) {
	// Given a repository with an existing session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(), Context: normalContext}
	require.NoError(t, repo.Create(ctx, session))

	// When deleting it
	err := repo.Delete(ctx, session.ID)

	// Then it no longer exists
	require.NoError(t, err)
	_, getErr := repo.GetByID(ctx, session.ID)
	assert.ErrorIs(t, getErr, study.ErrSessionNotFound)
}

func TestSessionRepository_Delete_returnsNotFound_whenSessionDoesNotExist(t *testing.T) {
	// Given a repository with no sessions
	repo := newTestSessionRepository(t)
	ctx := context.Background()

	// When deleting a session that does not exist
	err := repo.Delete(ctx, "missing-session")

	// Then it fails with ErrSessionNotFound
	assert.ErrorIs(t, err, study.ErrSessionNotFound)
}

func TestSessionRepository_ReassignFolder_movesEverySessionFromOneFolderToAnother(t *testing.T) {
	// Given two sessions in folder-a and one in folder-b
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-2", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC(), Context: normalContext}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-3", Mode: study.ModeStudy, FolderID: "folder-b", StartedAt: time.Now().UTC(), Context: normalContext}))

	// When reassigning folder-a's sessions to folder-b
	err := repo.ReassignFolder(ctx, "folder-a", "folder-b")

	// Then folder-a is empty and folder-b has all three sessions
	require.NoError(t, err)
	remaining, listErr := repo.ListByFolder(ctx, "folder-a")
	require.NoError(t, listErr)
	assert.Empty(t, remaining)
	moved, listErr := repo.ListByFolder(ctx, "folder-b")
	require.NoError(t, listErr)
	assert.Len(t, moved, 3)
}

func TestSessionRepository_UpdateContext_persistsNewUsage(t *testing.T) {
	// Given a repository with an existing session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(), Context: normalContext,
	}))

	// When updating its context usage
	newUsage := study.ContextUsage{
		State: study.ContextStateBlocked, Model: "anthropic/claude-sonnet-4.5",
		UsedTokens: 195000, ContextLength: 200000, Estimated: false,
	}
	err := repo.UpdateContext(ctx, "session-1", newUsage)

	// Then the new usage is persisted
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, "session-1")
	require.NoError(t, getErr)
	assert.Equal(t, newUsage, stored.Context)
}

func TestSessionRepository_UpdateContext_returnsNotFound_whenSessionDoesNotExist(t *testing.T) {
	// Given a repository with no sessions
	repo := newTestSessionRepository(t)
	ctx := context.Background()

	// When updating context for a session that does not exist
	err := repo.UpdateContext(ctx, "missing-session", normalContext)

	// Then it fails with ErrSessionNotFound
	assert.ErrorIs(t, err, study.ErrSessionNotFound)
}

func TestSessionRepository_UpdateContext_rejectsInvalidState(t *testing.T) {
	// Given a repository with an existing session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(), Context: normalContext,
	}))

	// When updating it with an unrecognized state
	err := repo.UpdateContext(ctx, "session-1", study.ContextUsage{State: "bogus"})

	// Then it is rejected before touching SQL
	require.Error(t, err)
}

func TestSessionRepository_UpdateContext_participatesInTransaction(t *testing.T) {
	// Given a repository with an existing session and a transactor
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewSessionRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{
		ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC(), Context: normalContext,
	}))
	transactor := NewSQLTransactor(db)

	// When an UpdateContext call inside a failed transaction is rolled back
	txErr := transactor.WithinTx(ctx, func(ctx context.Context) error {
		if err := repo.UpdateContext(ctx, "session-1", study.ContextUsage{State: study.ContextStateBlocked, UsedTokens: 1}); err != nil {
			return err
		}
		return assert.AnError
	})

	// Then the transaction fails and the update never landed
	require.Error(t, txErr)
	stored, getErr := repo.GetByID(ctx, "session-1")
	require.NoError(t, getErr)
	assert.Equal(t, normalContext, stored.Context)
}
