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

func TestSessionRepository_End_setsEndedAt(t *testing.T) {
	// Given a repository with an open session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{ID: "session-1", Topic: "Topic", Mode: study.ModeStudy, StartedAt: time.Now().UTC()}
	require.NoError(t, repo.Create(ctx, session))
	endedAt := time.Now().UTC().Truncate(time.Second)

	// When ending it
	err := repo.End(ctx, session.ID, endedAt)

	// Then it succeeds and ended_at is stored
	require.NoError(t, err)
	var storedEndedAt time.Time
	queryErr := repo.db.QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE id = ?`, session.ID,
	).Scan(&storedEndedAt)
	require.NoError(t, queryErr)
	assert.True(t, endedAt.Equal(storedEndedAt))
}

func TestSessionRepository_End_returnsNotFound_whenSessionDoesNotExist(t *testing.T) {
	// Given a repository with no sessions
	repo := newTestSessionRepository(t)
	ctx := context.Background()

	// When ending a session that does not exist
	err := repo.End(ctx, "missing-session", time.Now().UTC())

	// Then it fails with ErrSessionNotFound
	assert.ErrorIs(t, err, study.ErrSessionNotFound)
}

func TestSessionRepository_GetByID_returnsStoredSession(t *testing.T) {
	// Given a repository with an existing session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{
		ID: "session-1", Topic: "Topic", Mode: study.ModeStudy, FolderID: "default",
		StartedAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, repo.Create(ctx, session))

	// When fetching it by id
	stored, err := repo.GetByID(ctx, "session-1")

	// Then the stored fields are returned
	require.NoError(t, err)
	assert.Equal(t, session.Topic, stored.Topic)
	assert.Equal(t, session.FolderID, stored.FolderID)
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

func TestSessionRepository_ListByFolder_returnsOnlySessionsInThatFolder(t *testing.T) {
	// Given sessions spread across two folders
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC()}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-2", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC()}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-3", Mode: study.ModeStudy, FolderID: "folder-b", StartedAt: time.Now().UTC()}))

	// When listing sessions in folder-a
	sessions, err := repo.ListByFolder(ctx, "folder-a")

	// Then only its two sessions are returned
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestSessionRepository_Reopen_clearsEndedAt(t *testing.T) {
	// Given a repository with an ended session
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC()}
	require.NoError(t, repo.Create(ctx, session))
	require.NoError(t, repo.End(ctx, session.ID, time.Now().UTC()))

	// When reopening it
	err := repo.Reopen(ctx, session.ID)

	// Then it is open again
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, session.ID)
	require.NoError(t, getErr)
	assert.True(t, stored.IsOpen())
}

func TestSessionRepository_Reopen_returnsNotFound_whenSessionDoesNotExist(t *testing.T) {
	// Given a repository with no sessions
	repo := newTestSessionRepository(t)
	ctx := context.Background()

	// When reopening a session that does not exist
	err := repo.Reopen(ctx, "missing-session")

	// Then it fails with ErrSessionNotFound
	assert.ErrorIs(t, err, study.ErrSessionNotFound)
}

func TestSessionRepository_MoveToFolder_updatesFolderID(t *testing.T) {
	// Given a repository with a session in one folder
	repo := newTestSessionRepository(t)
	ctx := context.Background()
	session := study.Session{ID: "session-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC()}
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
	session := study.Session{ID: "session-1", Mode: study.ModeStudy, FolderID: "default", StartedAt: time.Now().UTC()}
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
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-1", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC()}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-2", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: time.Now().UTC()}))
	require.NoError(t, repo.Create(ctx, study.Session{ID: "s-3", Mode: study.ModeStudy, FolderID: "folder-b", StartedAt: time.Now().UTC()}))

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
