package exercise

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var attemptStart = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func attemptRubricVersion() RubricVersion {
	return RubricVersion{ID: "rubric-1", ChallengeID: "challenge-1", Version: 1}
}

func TestNewAttempt_setsDeadlineFromChallengeTimeLimit(t *testing.T) {
	// Given a challenge with a 45 minute limit and its rubric version
	challenge := validChallenge()
	challenge.TimeLimitMinutes = 45

	// When starting an attempt
	attempt, err := NewAttempt(challenge, attemptRubricVersion(), "session-exercise-1", attemptStart)

	// Then it is in progress, linked to the challenge, rubric and session link, and ends 45 minutes later
	require.NoError(t, err)
	assert.Equal(t, ChallengeAttempt{
		ChallengeID:       "challenge-1",
		RubricVersionID:   "rubric-1",
		SessionExerciseID: "session-exercise-1",
		State:             AttemptInProgress,
		StartedAt:         attemptStart,
		Deadline:          attemptStart.Add(45 * time.Minute),
	}, attempt)
}

func TestNewAttempt_allowsStandaloneAttempt(t *testing.T) {
	// Given no session link
	// When starting an attempt
	attempt, err := NewAttempt(validChallenge(), attemptRubricVersion(), "", attemptStart)

	// Then the link stays empty
	require.NoError(t, err)
	assert.Empty(t, attempt.SessionExerciseID)
}

func TestNewAttempt_rejectsRubricVersionOfAnotherChallenge(t *testing.T) {
	// Given a rubric version that belongs to a different challenge
	rubric := attemptRubricVersion()
	rubric.ChallengeID = "challenge-2"

	// When starting an attempt
	_, err := NewAttempt(validChallenge(), rubric, "", attemptStart)

	// Then it is refused
	assert.ErrorIs(t, err, ErrRubricChallengeMismatch)
}

func TestChallengeAttempt_Expired_isTrueFromTheDeadlineOn(t *testing.T) {
	// Given an attempt with a 45 minute deadline
	attempt, err := NewAttempt(validChallenge(), attemptRubricVersion(), "", attemptStart)
	require.NoError(t, err)
	deadline := attempt.Deadline

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"just before the deadline", deadline.Add(-time.Nanosecond), false},
		{"exactly at the deadline", deadline, true},
		{"after the deadline", deadline.Add(time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When asking whether it expired
			got := attempt.Expired(tt.now)

			// Then the deadline is inclusive
			assert.Equal(t, tt.want, got)
		})
	}
}

func inProgressAttempt(t *testing.T) ChallengeAttempt {
	t.Helper()
	attempt, err := NewAttempt(validChallenge(), attemptRubricVersion(), "", attemptStart)
	require.NoError(t, err)
	attempt.ID = "attempt-1"
	return attempt
}

func TestChallengeAttempt_Submit_movesToSubmittedAndReturnsSubmission(t *testing.T) {
	// Given an in-progress attempt
	attempt := inProgressAttempt(t)
	now := attemptStart.Add(10 * time.Minute)

	// When the user submits a diagram
	submitted, submission, err := attempt.Submit("application/json", "hello", SubmitReasonUser, now)

	// Then the attempt is SUBMITTED with the reason and time, and the submission carries the SHA-256 of the content
	require.NoError(t, err)
	assert.Equal(t, AttemptSubmitted, submitted.State)
	assert.Equal(t, SubmitReasonUser, submitted.SubmitReason)
	assert.Equal(t, now, submitted.SubmittedAt)
	assert.Equal(t, Submission{
		AttemptID:   "attempt-1",
		ContentType: "application/json",
		Content:     "hello",
		Checksum:    "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		SubmittedAt: now,
	}, submission)
	assert.Equal(t, AttemptInProgress, attempt.State, "the original value is not mutated")
}

func TestChallengeAttempt_Submit_acceptsTimeExpired(t *testing.T) {
	// Given an in-progress attempt
	attempt := inProgressAttempt(t)

	// When it is submitted because time ran out
	submitted, _, err := attempt.Submit("text/plain", "x", SubmitReasonTimeExpired, attempt.Deadline)

	// Then the reason is recorded
	require.NoError(t, err)
	assert.Equal(t, SubmitReasonTimeExpired, submitted.SubmitReason)
}

func TestChallengeAttempt_Submit_failsWhenNotInProgress(t *testing.T) {
	// Given an attempt that was already submitted
	attempt := inProgressAttempt(t)
	submitted, _, err := attempt.Submit("text/plain", "x", SubmitReasonUser, attemptStart)
	require.NoError(t, err)

	// When submitting it again
	_, _, err = submitted.Submit("text/plain", "y", SubmitReasonUser, attemptStart)

	// Then it fails
	assert.ErrorIs(t, err, ErrAttemptNotInProgress)
}

func TestChallengeAttempt_Submit_rejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		content     string
		reason      SubmitReason
		want        error
	}{
		{"empty content", "text/plain", "", SubmitReasonUser, ErrSubmissionContentRequired},
		{"blank content type", " ", "x", SubmitReasonUser, ErrSubmissionContentTypeRequired},
		{"oversized content", "text/plain", strings.Repeat("a", MaxSubmissionContentBytes+1), SubmitReasonUser, ErrSubmissionTooLarge},
		{"unknown reason", "text/plain", "x", "GAVE_UP", ErrSubmitReasonInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an in-progress attempt
			attempt := inProgressAttempt(t)

			// When submitting invalid input
			_, _, err := attempt.Submit(tt.contentType, tt.content, tt.reason, attemptStart)

			// Then the specific error is returned
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestChallengeAttempt_Submit_acceptsContentExactlyAtTheLimit(t *testing.T) {
	// Given content of exactly the maximum size
	attempt := inProgressAttempt(t)

	// When submitting it
	_, _, err := attempt.Submit("text/plain", strings.Repeat("a", MaxSubmissionContentBytes), SubmitReasonUser, attemptStart)

	// Then the limit is inclusive
	assert.NoError(t, err)
}

func attemptIn(state AttemptState) ChallengeAttempt {
	return ChallengeAttempt{ID: "attempt-1", State: state}
}

func TestChallengeAttempt_StartEvaluation_movesSubmittedAndFailedAttemptsToEvaluating(t *testing.T) {
	for _, from := range []AttemptState{AttemptSubmitted, AttemptEvaluationFailed} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt that can be evaluated, with an old failure reason
			attempt := attemptIn(from)
			attempt.FailureReason = "timeout"

			// When starting its evaluation
			started, err := attempt.StartEvaluation()

			// Then it is EVALUATING with no failure reason, and the receiver is untouched
			require.NoError(t, err)
			assert.Equal(t, AttemptEvaluating, started.State)
			assert.Empty(t, started.FailureReason)
			assert.Equal(t, from, attempt.State)
		})
	}
}

func TestChallengeAttempt_StartEvaluation_refusesEveryOtherState(t *testing.T) {
	for _, from := range []AttemptState{AttemptInProgress, AttemptEvaluating, AttemptEvaluated, AttemptFinished} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt that cannot be evaluated now
			// When starting its evaluation
			_, err := attemptIn(from).StartEvaluation()

			// Then it is refused
			assert.ErrorIs(t, err, ErrAttemptNotEvaluable)
		})
	}
}

func TestChallengeAttempt_CompleteEvaluation_movesEvaluatingToEvaluated(t *testing.T) {
	// Given an attempt being evaluated
	// When completing its evaluation
	completed, err := attemptIn(AttemptEvaluating).CompleteEvaluation()

	// Then it is EVALUATED
	require.NoError(t, err)
	assert.Equal(t, AttemptEvaluated, completed.State)
}

func TestChallengeAttempt_FailEvaluation_movesEvaluatingToFailedKeepingTheReason(t *testing.T) {
	// Given an attempt being evaluated
	// When its evaluation fails
	failed, err := attemptIn(AttemptEvaluating).FailEvaluation("the model timed out")

	// Then it is EVALUATION_FAILED with the reason
	require.NoError(t, err)
	assert.Equal(t, AttemptEvaluationFailed, failed.State)
	assert.Equal(t, "the model timed out", failed.FailureReason)
}

func TestChallengeAttempt_CompleteAndFailEvaluation_refuseAnAttemptThatIsNotEvaluating(t *testing.T) {
	for _, from := range []AttemptState{AttemptInProgress, AttemptSubmitted, AttemptEvaluated, AttemptEvaluationFailed, AttemptFinished} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt that is not being evaluated
			// When completing or failing its evaluation
			_, completeErr := attemptIn(from).CompleteEvaluation()
			_, failErr := attemptIn(from).FailEvaluation("x")

			// Then both are refused
			assert.ErrorIs(t, completeErr, ErrAttemptNotEvaluating)
			assert.ErrorIs(t, failErr, ErrAttemptNotEvaluating)
		})
	}
}
