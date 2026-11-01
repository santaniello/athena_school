package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

func newTestChallengeRepository(t *testing.T) (*ChallengeRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-a', 'Folder A', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	insertTestSession(t, db, "session-1", "caching")
	return NewChallengeRepository(db), db
}

func insertTestSession(t *testing.T, db *sql.DB, id, topic string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES (?, ?, 'study', 'folder-a', CURRENT_TIMESTAMP)`,
		id, topic,
	)
	require.NoError(t, err)
}

func newTestChallenge(id string) exercise.Challenge {
	at := time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC)
	return exercise.Challenge{
		ID:                        id,
		Type:                      exercise.ChallengeTypeArchitecture,
		Title:                     "Design a URL shortener",
		Scenario:                  "A startup needs a service that shortens URLs.",
		FunctionalRequirements:    []string{"Create a short link", "Redirect a short link"},
		NonFunctionalRequirements: []string{"99.9% availability", "p99 under 100ms"},
		Constraints:               []string{"Single region", "Small team"},
		Objectives:                []string{"Justify the storage choice", "Explain the cache"},
		Difficulty:                exercise.DifficultyIntermediate,
		Topic:                     "caching",
		SourceSessionID:           "session-1",
		Origin:                    exercise.OriginAIGenerated,
		CreatedAt:                 at,
		UpdatedAt:                 at,
	}
}

func TestChallengeRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given a repository and a challenge with every list populated
	repo, _ := newTestChallengeRepository(t)
	ctx := context.Background()
	challenge := newTestChallenge("challenge-1")

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, challenge))
	got, err := repo.Get(ctx, "challenge-1")

	// Then every field, including list order and timestamps, is preserved
	require.NoError(t, err)
	assert.Equal(t, challenge, got)
}

func TestChallengeRepository_CreateThenGet_returnsEmptyListsWhenOptionalListsAreNil(t *testing.T) {
	// Given a challenge without optional lists
	repo, _ := newTestChallengeRepository(t)
	ctx := context.Background()
	challenge := newTestChallenge("challenge-1")
	challenge.NonFunctionalRequirements = nil
	challenge.Constraints = nil

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, challenge))
	got, err := repo.Get(ctx, "challenge-1")

	// Then the optional lists are empty, not nil
	require.NoError(t, err)
	assert.Equal(t, []string{}, got.NonFunctionalRequirements)
	assert.Equal(t, []string{}, got.Constraints)
}

func TestChallengeRepository_Create_storesNoSourceSessionWhenEmpty(t *testing.T) {
	// Given a challenge with no source session
	repo, db := newTestChallengeRepository(t)
	ctx := context.Background()
	challenge := newTestChallenge("challenge-1")
	challenge.SourceSessionID = ""

	// When creating it
	require.NoError(t, repo.Create(ctx, challenge))

	// Then the column is NULL and the challenge reads back with an empty id
	var raw sql.NullString
	require.NoError(t, db.QueryRow(`SELECT source_session_id FROM challenges WHERE id = 'challenge-1'`).Scan(&raw))
	assert.False(t, raw.Valid)
	got, err := repo.Get(ctx, "challenge-1")
	require.NoError(t, err)
	assert.Empty(t, got.SourceSessionID)
}

func TestChallengeRepository_Create_failsForUnknownSourceSession(t *testing.T) {
	// Given a challenge pointing at a session that does not exist
	repo, _ := newTestChallengeRepository(t)
	challenge := newTestChallenge("challenge-1")
	challenge.SourceSessionID = "missing-session"

	// When creating it
	err := repo.Create(context.Background(), challenge)

	// Then the foreign key rejects it
	assert.Error(t, err)
}

func TestChallengeRepository_Create_failsForDuplicateID(t *testing.T) {
	// Given a stored challenge
	repo, _ := newTestChallengeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestChallenge("challenge-1")))

	// When creating another with the same id
	err := repo.Create(ctx, newTestChallenge("challenge-1"))

	// Then it fails instead of overwriting
	assert.Error(t, err)
}

func TestChallengeRepository_Get_returnsNotFoundForUnknownID(t *testing.T) {
	// Given an empty repository
	repo, _ := newTestChallengeRepository(t)

	// When getting an unknown id
	_, err := repo.Get(context.Background(), "missing")

	// Then ErrChallengeNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrChallengeNotFound)
}
