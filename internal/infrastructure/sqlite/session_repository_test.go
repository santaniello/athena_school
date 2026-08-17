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
		StartedAt: time.Now().UTC().Truncate(time.Second),
	}

	// When creating it
	err := repo.Create(ctx, session)

	// Then it succeeds and the row is queryable
	require.NoError(t, err)
	var topic, mode string
	queryErr := repo.db.QueryRowContext(ctx,
		`SELECT topic, mode FROM sessions WHERE id = ?`, session.ID,
	).Scan(&topic, &mode)
	require.NoError(t, queryErr)
	assert.Equal(t, session.Topic, topic)
	assert.Equal(t, session.Mode, mode)
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
