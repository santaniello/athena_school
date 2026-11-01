package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

func newTestSessionExerciseRepository(t *testing.T) (*SessionExerciseRepository, *sql.DB) {
	t.Helper()
	_, db := newTestAttemptRepository(t)
	_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-1', 'session-1', 'assistant', 'x', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return NewSessionExerciseRepository(db), db
}

func newTestSessionExercise(id string) exercise.SessionExercise {
	return exercise.SessionExercise{
		ID:          id,
		SessionID:   "session-1",
		ChallengeID: "challenge-1",
		MessageID:   "message-1",
		CreatedAt:   time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC),
	}
}

func TestSessionExerciseRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given a link between a session and a challenge
	repo, _ := newTestSessionExerciseRepository(t)
	ctx := context.Background()
	link := newTestSessionExercise("link-1")

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, link))
	got, err := repo.GetBySessionAndChallenge(ctx, "session-1", "challenge-1")

	// Then every field is preserved
	require.NoError(t, err)
	assert.Equal(t, link, got)
}

func TestSessionExerciseRepository_Create_failsForTheSameSessionAndChallenge(t *testing.T) {
	// Given a session already linked to the challenge
	repo, _ := newTestSessionExerciseRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestSessionExercise("link-1")))

	// When linking them a second time
	err := repo.Create(ctx, newTestSessionExercise("link-2"))

	// Then ErrSessionExerciseExists is returned
	assert.ErrorIs(t, err, exercise.ErrSessionExerciseExists)
}

func TestSessionExerciseRepository_Create_doesNotReportAnIDClashAsExisting(t *testing.T) {
	// Given a stored link
	repo, db := newTestSessionExerciseRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestSessionExercise("link-1")))
	insertTestSession(t, db, "session-2", "caching")

	// When another session's link reuses the id
	other := newTestSessionExercise("link-1")
	other.SessionID = "session-2"
	err := repo.Create(ctx, other)

	// Then it fails, but it is not the "already linked" case
	require.Error(t, err)
	assert.NotErrorIs(t, err, exercise.ErrSessionExerciseExists)
}

func TestSessionExerciseRepository_GetBySessionAndChallenge_returnsNotFound(t *testing.T) {
	// Given no link
	repo, _ := newTestSessionExerciseRepository(t)

	// When getting one
	_, err := repo.GetBySessionAndChallenge(context.Background(), "session-1", "challenge-1")

	// Then ErrSessionExerciseNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrSessionExerciseNotFound)
}

func TestSessionExerciseRepository_ListBySession_returnsOnlyThatSessionsLinksOldestFirst(t *testing.T) {
	// Given two links in session-1 and one in session-2
	repo, db := newTestSessionExerciseRepository(t)
	ctx := context.Background()
	challenges := NewChallengeRepository(db)
	require.NoError(t, challenges.Create(ctx, newTestChallenge("challenge-2")))
	insertTestSession(t, db, "session-2", "caching")

	newer := newTestSessionExercise("link-a")
	newer.ChallengeID = "challenge-2"
	newer.CreatedAt = newer.CreatedAt.Add(time.Hour)
	older := newTestSessionExercise("link-z")
	require.NoError(t, repo.Create(ctx, newer))
	require.NoError(t, repo.Create(ctx, older))
	_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-2', 'session-2', 'assistant', 'x', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	foreign := exercise.SessionExercise{ID: "link-other", SessionID: "session-2", ChallengeID: "challenge-1", MessageID: "message-2", CreatedAt: older.CreatedAt}
	require.NoError(t, repo.Create(ctx, foreign))

	// When listing session-1's links
	got, err := repo.ListBySession(ctx, "session-1")

	// Then only its two links come back, oldest first
	require.NoError(t, err)
	assert.Equal(t, []exercise.SessionExercise{older, newer}, got)
}

func TestSessionExerciseRepository_ListBySession_returnsEmptyListWithoutLinks(t *testing.T) {
	// Given a session with no links
	repo, _ := newTestSessionExerciseRepository(t)

	// When listing
	got, err := repo.ListBySession(context.Background(), "session-1")

	// Then the list is empty, not nil
	require.NoError(t, err)
	assert.Equal(t, []exercise.SessionExercise{}, got)
}

func TestSubmissionRepository_GetByAttempt_returnsTheStoredSubmission(t *testing.T) {
	// Given a submitted attempt
	attempts, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, attempts.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, attempts.Submit(ctx, submitted, submission))

	// When reading its submission
	got, err := NewSubmissionRepository(db).GetByAttempt(ctx, "attempt-1")

	// Then every field is preserved
	require.NoError(t, err)
	assert.Equal(t, submission, got)
}

func TestSubmissionRepository_GetByAttempt_returnsNotFoundForAnAttemptWithoutSubmission(t *testing.T) {
	// Given an attempt still in progress
	attempts, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, attempts.Create(ctx, newTestAttempt("attempt-1")))

	// When reading its submission
	_, err := NewSubmissionRepository(db).GetByAttempt(ctx, "attempt-1")

	// Then ErrSubmissionNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrSubmissionNotFound)
}

func TestSessionRepository_Delete_removesLinksButKeepsAttemptsSubmissionsAndChallenges(t *testing.T) {
	// Given a session linked to a challenge, with a submitted attempt that came from the link
	links, db := newTestSessionExerciseRepository(t)
	ctx := context.Background()
	require.NoError(t, links.Create(ctx, newTestSessionExercise("link-1")))
	attempts := NewAttemptRepository(db)
	attempt := newTestAttempt("attempt-1")
	attempt.SessionExerciseID = "link-1"
	require.NoError(t, attempts.Create(ctx, attempt))
	submitted, submission := submitForTest(t, attempt, "submission-1")
	require.NoError(t, attempts.Submit(ctx, submitted, submission))

	// When the session is deleted
	require.NoError(t, NewSessionRepository(db).Delete(ctx, "session-1"))

	// Then its links are gone, while the attempt (now standalone), its submission and the challenge remain
	assert.Equal(t, 0, countTableRows(t, db, "session_exercises"))
	got, err := attempts.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Empty(t, got.SessionExerciseID)
	assert.Equal(t, exercise.AttemptSubmitted, got.State)
	stored, err := NewSubmissionRepository(db).GetByAttempt(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, submission, stored)
	_, err = NewChallengeRepository(db).Get(ctx, "challenge-1")
	assert.NoError(t, err)
}
