package exercise

import (
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
