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

var attemptTestStart = time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC)

// newTestAttemptRepository returns a repository over a database that already
// holds challenge-1 and its rubric version rubric-1.
func newTestAttemptRepository(t *testing.T) (*AttemptRepository, *sql.DB) {
	t.Helper()
	rubrics, db := newTestRubricRepository(t)
	require.NoError(t, rubrics.Create(context.Background(), newTestRubricVersion("rubric-1", 1)))
	return NewAttemptRepository(db), db
}

func newTestAttempt(id string) exercise.ChallengeAttempt {
	return exercise.ChallengeAttempt{
		ID:              id,
		ChallengeID:     "challenge-1",
		RubricVersionID: "rubric-1",
		State:           exercise.AttemptInProgress,
		StartedAt:       attemptTestStart,
		Deadline:        attemptTestStart.Add(45 * time.Minute),
	}
}

// insertTestSessionExercise stores a link for session-1 and challenge-1 under
// a fresh assistant message.
func insertTestSessionExercise(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, 'session-1', 'assistant', 'x', CURRENT_TIMESTAMP)`, "message-"+id)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO session_exercises (id, session_id, challenge_id, message_id, created_at)
		VALUES (?, 'session-1', 'challenge-1', ?, 'now')`, id, "message-"+id)
	require.NoError(t, err)
}

func TestAttemptRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given a submitted attempt that came from a session link
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	insertTestSessionExercise(t, db, "link-1")
	attempt := newTestAttempt("attempt-1")
	attempt.SessionExerciseID = "link-1"
	attempt.State = exercise.AttemptSubmitted
	attempt.SubmittedAt = attemptTestStart.Add(10 * time.Minute)
	attempt.SubmitReason = exercise.SubmitReasonTimeExpired

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, attempt))
	got, err := repo.Get(ctx, "attempt-1")

	// Then every field is preserved
	require.NoError(t, err)
	assert.Equal(t, attempt, got)
}

func TestAttemptRepository_CreateThenGet_keepsStandaloneAttemptWithoutOptionalFields(t *testing.T) {
	// Given a standalone in-progress attempt
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attempt := newTestAttempt("attempt-1")

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, attempt))
	got, err := repo.Get(ctx, "attempt-1")

	// Then the link, submit time and reason stay empty
	require.NoError(t, err)
	assert.Equal(t, attempt, got)
	assert.Empty(t, got.SessionExerciseID)
	assert.True(t, got.SubmittedAt.IsZero())
}

func TestAttemptRepository_Get_returnsNotFound(t *testing.T) {
	// Given an empty repository
	repo, _ := newTestAttemptRepository(t)

	// When getting an unknown attempt
	_, err := repo.Get(context.Background(), "missing")

	// Then ErrAttemptNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrAttemptNotFound)
}

func TestAttemptRepository_Create_duplicateIDIsNotReportedAsAnotherAttemptInProgress(t *testing.T) {
	// Given a submitted attempt, so no attempt is running
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	first := newTestAttempt("attempt-1")
	require.NoError(t, repo.Create(ctx, first))
	submitted, submission, err := first.Submit("text/plain", "diagram", exercise.SubmitReasonUser, first.StartedAt)
	require.NoError(t, err)
	submission.ID = "submission-1"
	require.NoError(t, repo.Submit(ctx, submitted, submission))

	// When storing an attempt with the same id
	err = repo.Create(ctx, newTestAttempt("attempt-1"))

	// Then it fails, but not as a running-exam conflict
	require.Error(t, err)
	assert.NotErrorIs(t, err, exercise.ErrAnotherAttemptInProgress)
}

func TestAttemptRepository_Create_refusesASecondInProgressAttempt(t *testing.T) {
	// Given an attempt already in progress
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))

	// When storing another in-progress attempt
	err := repo.Create(ctx, newTestAttempt("attempt-2"))

	// Then the unique index refuses it, reported as the domain's conflict error
	assert.ErrorIs(t, err, exercise.ErrAnotherAttemptInProgress)
	_, getErr := repo.Get(ctx, "attempt-2")
	assert.ErrorIs(t, getErr, exercise.ErrAttemptNotFound)
}

func TestAttemptRepository_Create_allowsANewAttemptOnceTheFirstIsSubmitted(t *testing.T) {
	// Given a challenge whose first attempt was submitted
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	first := newTestAttempt("attempt-1")
	require.NoError(t, repo.Create(ctx, first))
	submitted, submission := submitForTest(t, first, "submission-1")
	require.NoError(t, repo.Submit(ctx, submitted, submission))

	// When practicing again
	err := repo.Create(ctx, newTestAttempt("attempt-2"))

	// Then a new in-progress attempt is stored
	assert.NoError(t, err)
}

func submitForTest(t *testing.T, attempt exercise.ChallengeAttempt, submissionID string) (exercise.ChallengeAttempt, exercise.Submission) {
	t.Helper()
	submitted, submission, err := attempt.Submit("application/json", `{"nodes":[]}`, exercise.SubmitReasonUser, attemptTestStart.Add(10*time.Minute))
	require.NoError(t, err)
	submission.ID = submissionID
	return submitted, submission
}

func TestAttemptRepository_Submit_storesSubmissionAndMovesAttemptToSubmitted(t *testing.T) {
	// Given an in-progress attempt
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")

	// When submitting it
	require.NoError(t, repo.Submit(ctx, submitted, submission))

	// Then the attempt is SUBMITTED with its reason and time, and the submission is stored
	got, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, submitted, got)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE attempt_id = 'attempt-1'`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestAttemptRepository_Submit_secondSubmitFailsAndStoresNothing(t *testing.T) {
	// Given an attempt that was already submitted
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, repo.Submit(ctx, submitted, submission))
	again, secondSubmission := submitForTest(t, newTestAttempt("attempt-1"), "submission-2")
	again.SubmitReason = exercise.SubmitReasonTimeExpired

	// When submitting it again
	err := repo.Submit(ctx, again, secondSubmission)

	// Then it fails, and neither the attempt nor the submissions change
	assert.ErrorIs(t, err, exercise.ErrAttemptNotInProgress)
	got, getErr := repo.Get(ctx, "attempt-1")
	require.NoError(t, getErr)
	assert.Equal(t, exercise.SubmitReasonUser, got.SubmitReason)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestAttemptRepository_Submit_rollsBackTheStateWhenTheSubmissionCannotBeStored(t *testing.T) {
	// Given a submission id that is already taken by another attempt's submission
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	first, firstSubmission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, repo.Submit(ctx, first, firstSubmission))
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-2")))
	second, secondSubmission := submitForTest(t, newTestAttempt("attempt-2"), "submission-1")

	// When submitting the second attempt
	err := repo.Submit(ctx, second, secondSubmission)

	// Then the insert fails and the attempt is still IN_PROGRESS
	require.Error(t, err)
	got, getErr := repo.Get(ctx, "attempt-2")
	require.NoError(t, getErr)
	assert.Equal(t, exercise.AttemptInProgress, got.State)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestAttemptRepository_ListByChallenge_returnsEveryAttemptOldestFirst(t *testing.T) {
	// Given a challenge practiced twice
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	older := newTestAttempt("attempt-z")
	older.StartedAt = attemptTestStart
	require.NoError(t, repo.Create(ctx, older))
	submittedOlder, submission := submitForTest(t, older, "submission-1")
	require.NoError(t, repo.Submit(ctx, submittedOlder, submission))
	newer := newTestAttempt("attempt-a")
	newer.StartedAt = attemptTestStart.Add(time.Hour)
	newer.Deadline = newer.StartedAt.Add(45 * time.Minute)
	require.NoError(t, repo.Create(ctx, newer))

	// When listing the challenge's attempts
	got, err := repo.ListByChallenge(ctx, "challenge-1")

	// Then both come back, oldest first
	require.NoError(t, err)
	assert.Equal(t, []exercise.ChallengeAttempt{submittedOlder, newer}, got)
}

func TestAttemptRepository_ListByChallenge_returnsEmptyListForAChallengeWithoutAttempts(t *testing.T) {
	// Given no attempts
	repo, _ := newTestAttemptRepository(t)

	// When listing
	got, err := repo.ListByChallenge(context.Background(), "challenge-1")

	// Then the list is empty, not nil
	require.NoError(t, err)
	assert.Equal(t, []exercise.ChallengeAttempt{}, got)
}

func TestAttemptRepository_FindInProgress_returnsTheRunningAttempt(t *testing.T) {
	// Given a submitted attempt and a running one
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	done := newTestAttempt("attempt-1")
	require.NoError(t, repo.Create(ctx, done))
	submitted, submission := submitForTest(t, done, "submission-1")
	require.NoError(t, repo.Submit(ctx, submitted, submission))
	running := newTestAttempt("attempt-2")
	require.NoError(t, repo.Create(ctx, running))

	// When finding the one in progress
	got, err := repo.FindInProgress(ctx)

	// Then it is the running one
	require.NoError(t, err)
	assert.Equal(t, running, got)
}

func TestAttemptRepository_FindInProgress_returnsNotFoundWhenNoneRuns(t *testing.T) {
	// Given no attempt in progress
	repo, _ := newTestAttemptRepository(t)

	// When finding it
	_, err := repo.FindInProgress(context.Background())

	// Then ErrAttemptNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrAttemptNotFound)
}

// insertFinishedAttempt stores an attempt in the FINISHED state, which the
// domain only gains in spec 3.12, together with a submission.
func insertFinishedAttempt(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO challenge_attempts (id, challenge_id, rubric_version_id, state, started_at, deadline, submitted_at, submit_reason)
		VALUES (?, 'challenge-1', 'rubric-1', 'FINISHED', ?, ?, ?, 'USER')`,
		id, formatChallengeTime(attemptTestStart), formatChallengeTime(attemptTestStart), formatChallengeTime(attemptTestStart))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO submissions (id, attempt_id, content_type, content, checksum, submitted_at)
		VALUES (?, ?, 'text/plain', 'x', 'y', 'z')`, "submission-"+id, id)
	require.NoError(t, err)
}

func countTableRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count))
	return count
}

func TestAttemptRepository_DeleteUnfinished_removesAnInProgressAttempt(t *testing.T) {
	// Given an in-progress attempt
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))

	// When discarding it
	require.NoError(t, repo.DeleteUnfinished(ctx, "attempt-1"))

	// Then it is gone
	_, err := repo.Get(ctx, "attempt-1")
	assert.ErrorIs(t, err, exercise.ErrAttemptNotFound)
}

func TestAttemptRepository_DeleteUnfinished_removesASubmittedAttemptWithItsSubmission(t *testing.T) {
	// Given a submitted attempt
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, repo.Submit(ctx, submitted, submission))

	// When discarding it
	require.NoError(t, repo.DeleteUnfinished(ctx, "attempt-1"))

	// Then the attempt and its submission are gone
	_, err := repo.Get(ctx, "attempt-1")
	assert.ErrorIs(t, err, exercise.ErrAttemptNotFound)
	assert.Equal(t, 0, countTableRows(t, db, "submissions"))
}

func TestAttemptRepository_DeleteUnfinished_neverTouchesAFinishedAttempt(t *testing.T) {
	// Given a finished attempt with its submission
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	insertFinishedAttempt(t, db, "finished-1")

	// When asked to discard it
	require.NoError(t, repo.DeleteUnfinished(ctx, "finished-1"))

	// Then both rows remain
	assert.Equal(t, 1, countTableRows(t, db, "challenge_attempts"))
	assert.Equal(t, 1, countTableRows(t, db, "submissions"))
}

func TestAttemptRepository_DeleteUnfinished_onlyRemovesTheGivenAttempt(t *testing.T) {
	// Given a submitted attempt and a running one
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, repo.Submit(ctx, submitted, submission))
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-2")))

	// When discarding the running one
	require.NoError(t, repo.DeleteUnfinished(ctx, "attempt-2"))

	// Then the submitted attempt and its submission stay
	_, err := repo.Get(ctx, "attempt-1")
	assert.NoError(t, err)
	assert.Equal(t, 1, countTableRows(t, db, "submissions"))
}

func TestAttemptRepository_DeleteAllUnfinished_removesEveryUnfinishedAttemptAndKeepsFinishedOnes(t *testing.T) {
	// Given a submitted attempt, a running one and a finished one
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, repo.Submit(ctx, submitted, submission))
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-2")))
	insertFinishedAttempt(t, db, "finished-1")

	// When discarding all unfinished attempts
	removed, err := repo.DeleteAllUnfinished(ctx)

	// Then two were removed, with the unfinished submission, and the finished one stays
	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	assert.Equal(t, 1, countTableRows(t, db, "challenge_attempts"))
	assert.Equal(t, 1, countTableRows(t, db, "submissions"))
	_, err = repo.Get(ctx, "finished-1")
	assert.NoError(t, err)
}

func TestAttemptRepository_DeleteAllUnfinished_returnsZeroWhenThereIsNothingToRemove(t *testing.T) {
	// Given no attempts
	repo, _ := newTestAttemptRepository(t)

	// When discarding all unfinished attempts
	removed, err := repo.DeleteAllUnfinished(context.Background())

	// Then nothing is removed
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestAttemptRepository_Submit_refusesAnAttemptThatIsNotSubmitted(t *testing.T) {
	// Given an in-progress attempt and a valid submission, but the attempt passed is still IN_PROGRESS
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	_, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")

	// When submitting it
	err := repo.Submit(ctx, newTestAttempt("attempt-1"), submission)

	// Then it is refused and nothing is stored, so the attempt can still be submitted properly
	assert.ErrorIs(t, err, exercise.ErrInvalidSubmitTransition)
	assert.Equal(t, 0, countTableRows(t, db, "submissions"))
	submitted, validSubmission := submitForTest(t, newTestAttempt("attempt-1"), "submission-2")
	assert.NoError(t, repo.Submit(ctx, submitted, validSubmission))
}

func TestAttemptRepository_Submit_refusesASubmissionOfAnotherAttempt(t *testing.T) {
	// Given a submitted attempt value paired with a submission that names a different attempt
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	submission.AttemptID = "attempt-2"

	// When submitting them
	err := repo.Submit(ctx, submitted, submission)

	// Then it is refused, the attempt stays IN_PROGRESS and nothing is stored
	assert.ErrorIs(t, err, exercise.ErrInvalidSubmitTransition)
	got, getErr := repo.Get(ctx, "attempt-1")
	require.NoError(t, getErr)
	assert.Equal(t, exercise.AttemptInProgress, got.State)
	assert.Equal(t, 0, countTableRows(t, db, "submissions"))
}

// attemptInState stores attempt-1 as SUBMITTED, then moves it through the
// repository to the wanted evaluation state.
func attemptInState(t *testing.T, repo *AttemptRepository, id string, state exercise.AttemptState) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestAttempt(id)))
	submitted, submission := submitForTest(t, newTestAttempt(id), "submission-"+id)
	require.NoError(t, repo.Submit(ctx, submitted, submission))
	if state == exercise.AttemptSubmitted {
		return
	}
	require.NoError(t, repo.StartEvaluation(ctx, id))
	switch state {
	case exercise.AttemptEvaluated:
		require.NoError(t, repo.CompleteEvaluation(ctx, id, 80))
	case exercise.AttemptEvaluationFailed:
		require.NoError(t, repo.FailEvaluation(ctx, id, "model timeout"))
	}
}

func TestAttemptRepository_StartEvaluation_movesSubmittedToEvaluating(t *testing.T) {
	// Given a submitted attempt
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptSubmitted)

	// When starting its evaluation
	require.NoError(t, repo.StartEvaluation(ctx, "attempt-1"))

	// Then it is EVALUATING
	got, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, exercise.AttemptEvaluating, got.State)
}

func TestAttemptRepository_StartEvaluation_secondCallIsRefused(t *testing.T) {
	// Given an attempt already being evaluated
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptEvaluating)

	// When starting it again
	err := repo.StartEvaluation(ctx, "attempt-1")

	// Then no row changes and it is refused
	assert.ErrorIs(t, err, exercise.ErrAttemptNotEvaluable)
}

func TestAttemptRepository_StartEvaluation_refusesEvaluatedAndUnknownAttempts(t *testing.T) {
	// Given an evaluated attempt
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptEvaluated)

	// When starting its evaluation, or one of an unknown id
	evaluatedErr := repo.StartEvaluation(ctx, "attempt-1")
	unknownErr := repo.StartEvaluation(ctx, "nope")

	// Then both are refused
	assert.ErrorIs(t, evaluatedErr, exercise.ErrAttemptNotEvaluable)
	assert.ErrorIs(t, unknownErr, exercise.ErrAttemptNotEvaluable)
}

func TestAttemptRepository_FailEvaluation_keepsTheReasonUntilTheNextRetry(t *testing.T) {
	// Given an attempt whose evaluation failed
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptEvaluationFailed)
	failed, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, exercise.AttemptEvaluationFailed, failed.State)
	assert.Equal(t, "model timeout", failed.FailureReason)

	// When retrying
	require.NoError(t, repo.StartEvaluation(ctx, "attempt-1"))

	// Then it is EVALUATING again with the reason cleared
	retried, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, exercise.AttemptEvaluating, retried.State)
	assert.Empty(t, retried.FailureReason)
}

func TestAttemptRepository_CompleteAndFailEvaluation_onlyMatchAnEvaluatingAttempt(t *testing.T) {
	// Given a submitted attempt that is not being evaluated
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptSubmitted)

	// When completing or failing its evaluation
	completeErr := repo.CompleteEvaluation(ctx, "attempt-1", 80)
	failErr := repo.FailEvaluation(ctx, "attempt-1", "x")

	// Then both are refused and the state is unchanged
	assert.ErrorIs(t, completeErr, exercise.ErrAttemptNotEvaluating)
	assert.ErrorIs(t, failErr, exercise.ErrAttemptNotEvaluating)
	got, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, exercise.AttemptSubmitted, got.State)
}

func TestAttemptRepository_DeleteUnfinishedEvaluations_removesEvaluatingAndFailedAttemptsOnly(t *testing.T) {
	// Given attempts submitted, evaluating, failed and evaluated
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "submitted", exercise.AttemptSubmitted)
	attemptInState(t, repo, "evaluating", exercise.AttemptEvaluating)
	attemptInState(t, repo, "failed", exercise.AttemptEvaluationFailed)
	attemptInState(t, repo, "evaluated", exercise.AttemptEvaluated)

	// When discarding the unfinished evaluations
	removed, err := repo.DeleteUnfinishedEvaluations(ctx)

	// Then the evaluating and failed ones are gone with their submissions
	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	for _, id := range []string{"evaluating", "failed"} {
		_, err := repo.Get(ctx, id)
		assert.ErrorIs(t, err, exercise.ErrAttemptNotFound, id)
	}
	for _, id := range []string{"submitted", "evaluated"} {
		_, err := repo.Get(ctx, id)
		assert.NoError(t, err, id)
	}
	assert.Equal(t, 2, countTableRows(t, db, "submissions"))
}

func TestAttemptRepository_DeleteUnfinished_removesAnEvaluatedAttemptWithItsEvaluation(t *testing.T) {
	// Given an evaluated attempt with a stored evaluation
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptEvaluated)
	require.NoError(t, NewEvaluationRepository(db).Create(ctx, newTestEvaluation("evaluation-1")))

	// When discarding the attempt
	require.NoError(t, repo.DeleteUnfinished(ctx, "attempt-1"))

	// Then the evaluation and its criterion results go with it
	assert.Equal(t, 0, countTableRows(t, db, "challenge_attempts"))
	assert.Equal(t, 0, countTableRows(t, db, "evaluations"))
	assert.Equal(t, 0, countTableRows(t, db, "criterion_results"))
}

func TestAttemptRepository_DeleteAllUnfinished_removesEvaluationsOfEvaluatedAttempts(t *testing.T) {
	// Given an evaluated attempt with a stored evaluation
	repo, db := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptEvaluated)
	require.NoError(t, NewEvaluationRepository(db).Create(ctx, newTestEvaluation("evaluation-1")))

	// When cleaning every unfinished attempt
	removed, err := repo.DeleteAllUnfinished(ctx)

	// Then the attempt goes with its evaluation instead of failing on the foreign key
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, countTableRows(t, db, "evaluations"))
	assert.Equal(t, 0, countTableRows(t, db, "criterion_results"))
}

func TestAttemptRepository_CompleteEvaluation_freezesThePassScoreOnTheAttempt(t *testing.T) {
	// Given a submitted attempt, whose pass score is not known yet
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptSubmitted)
	before, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Zero(t, before.PassScore)

	// When its evaluation completes under a minimum of 65
	require.NoError(t, repo.StartEvaluation(ctx, "attempt-1"))
	require.NoError(t, repo.CompleteEvaluation(ctx, "attempt-1", 65))

	// Then the attempt keeps that minimum
	after, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, 65, after.PassScore)
	assert.Equal(t, exercise.AttemptEvaluated, after.State)
}

func TestAttemptRepository_StartEvaluation_leavesThePassScoreUnsetAfterAFailedRetry(t *testing.T) {
	// Given an attempt whose evaluation failed
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attemptInState(t, repo, "attempt-1", exercise.AttemptEvaluationFailed)

	// When reading it
	got, err := repo.Get(ctx, "attempt-1")

	// Then no minimum was frozen
	require.NoError(t, err)
	assert.Zero(t, got.PassScore)
}

func TestAttemptRepository_Create_storesAFrozenPassScoreOfAnAttemptCreatedWithOne(t *testing.T) {
	repo, _ := newTestAttemptRepository(t)
	ctx := context.Background()
	attempt := newTestAttempt("attempt-1")
	attempt.PassScore = 90

	require.NoError(t, repo.Create(ctx, attempt))

	got, err := repo.Get(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, 90, got.PassScore)
}
