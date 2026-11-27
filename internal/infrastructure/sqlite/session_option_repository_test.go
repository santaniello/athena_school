package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
	"github.com/santaniello/athena/internal/domain/study"
)

func TestSessionOptionRepository_ListArchLabEnabled_returnsSessionsOfEnabledFoldersNewestFirst(t *testing.T) {
	// Given sessions started at different times in two folders
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at, arch_lab_enabled) VALUES
		('folder-a', 'Backend', CURRENT_TIMESTAMP, 1), ('folder-b', 'Infra', CURRENT_TIMESTAMP, 1)`)
	require.NoError(t, err)
	sessions := NewSessionRepository(db)
	ctx := context.Background()
	oldest := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	middle := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	for _, s := range []study.Session{
		{ID: "s-old", Topic: "Queues", Mode: study.ModeStudy, FolderID: "folder-b", StartedAt: oldest, Context: normalContext},
		{ID: "s-new", Topic: "Caching", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: newest, Context: normalContext},
		{ID: "s-mid", Topic: "Sharding", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: middle, Context: normalContext},
	} {
		require.NoError(t, sessions.Create(ctx, s))
	}
	repo := NewSessionOptionRepository(db)

	// When listing every session
	got, err := repo.ListArchLabEnabled(ctx)

	// Then they come newest first, each with its topic and folder name
	require.NoError(t, err)
	assert.Equal(t, []exercise.SessionOption{
		{SessionID: "s-new", Topic: "Caching", FolderName: "Backend", StartedAt: newest},
		{SessionID: "s-mid", Topic: "Sharding", FolderName: "Backend", StartedAt: middle},
		{SessionID: "s-old", Topic: "Queues", FolderName: "Infra", StartedAt: oldest},
	}, got)
}

func TestSessionOptionRepository_ListArchLabEnabled_returnsAnEmptyListWithoutSessions(t *testing.T) {
	// Given a database with no sessions
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// When listing every session
	got, err := NewSessionOptionRepository(db).ListArchLabEnabled(context.Background())

	// Then the list is empty and not nil
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSessionOptionRepository_ListArchLabEnabled_skipsSessionsThatAreNotStudySessions(t *testing.T) {
	// Given a study session and a session of another mode
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at, arch_lab_enabled) VALUES ('folder-a', 'Backend', CURRENT_TIMESTAMP, 1)`)
	require.NoError(t, err)
	sessions := NewSessionRepository(db)
	ctx := context.Background()
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	require.NoError(t, sessions.Create(ctx, study.Session{ID: "s-study", Topic: "Caching", Mode: study.ModeStudy, FolderID: "folder-a", StartedAt: at, Context: normalContext}))
	require.NoError(t, sessions.Create(ctx, study.Session{ID: "s-other", Topic: "Mock", Mode: "interview", FolderID: "folder-a", StartedAt: at, Context: normalContext}))

	// When listing the options
	got, err := NewSessionOptionRepository(db).ListArchLabEnabled(ctx)

	// Then only the study session is offered
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "s-study", got[0].SessionID)
}

func TestSessionOptionRepository_ListArchLabEnabled_skipsSessionsOfDisabledFolders(t *testing.T) {
	// Given a session in an enabled folder and one in a disabled folder
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at, arch_lab_enabled) VALUES
		('folder-on', 'Architecture', CURRENT_TIMESTAMP, 1), ('folder-off', 'Java', CURRENT_TIMESTAMP, 0)`)
	require.NoError(t, err)
	sessions := NewSessionRepository(db)
	ctx := context.Background()
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	require.NoError(t, sessions.Create(ctx, study.Session{ID: "s-on", Topic: "Caching", Mode: study.ModeStudy, FolderID: "folder-on", StartedAt: at, Context: normalContext}))
	require.NoError(t, sessions.Create(ctx, study.Session{ID: "s-off", Topic: "Streams", Mode: study.ModeStudy, FolderID: "folder-off", StartedAt: at, Context: normalContext}))

	// When listing the options
	got, err := NewSessionOptionRepository(db).ListArchLabEnabled(ctx)

	// Then only the session of the enabled folder is offered
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "s-on", got[0].SessionID)
}
