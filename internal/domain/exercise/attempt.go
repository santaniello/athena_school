package exercise

import (
	"errors"
	"time"
)

// ErrRubricChallengeMismatch is returned when an attempt is started with a
// rubric version that belongs to another challenge.
var ErrRubricChallengeMismatch = errors.New("rubric version does not belong to the challenge")

// AttemptState is where an attempt is in its lifecycle. Evaluation and defense
// states are added by later specs.
type AttemptState string

// The attempt states defined so far.
const (
	AttemptInProgress AttemptState = "IN_PROGRESS"
	AttemptSubmitted  AttemptState = "SUBMITTED"
)

// SubmitReason records why an attempt ended.
type SubmitReason string

// The two reasons an attempt ends.
const (
	SubmitReasonUser        SubmitReason = "USER"
	SubmitReasonTimeExpired SubmitReason = "TIME_EXPIRED"
)

// ChallengeAttempt is one exam sitting on a challenge. The backend owns the
// clock: Deadline is fixed when the attempt starts, so a later edit of the
// challenge's time limit never changes it. See
// specs/phases/phase-03-architecture-lab/03-attempts-and-session-link.md.
type ChallengeAttempt struct {
	ID                string
	ChallengeID       string
	RubricVersionID   string // the version frozen when the attempt started
	SessionExerciseID string // "" for a standalone attempt
	State             AttemptState
	StartedAt         time.Time
	Deadline          time.Time
	SubmittedAt       time.Time // zero until submitted
	SubmitReason      SubmitReason
}

// NewAttempt starts an in-progress attempt on the challenge, frozen to the
// given rubric version, ending TimeLimitMinutes after now. The caller assigns
// the ID.
func NewAttempt(challenge Challenge, rubric RubricVersion, sessionExerciseID string, now time.Time) (ChallengeAttempt, error) {
	if rubric.ChallengeID != challenge.ID {
		return ChallengeAttempt{}, ErrRubricChallengeMismatch
	}
	return ChallengeAttempt{
		ChallengeID:       challenge.ID,
		RubricVersionID:   rubric.ID,
		SessionExerciseID: sessionExerciseID,
		State:             AttemptInProgress,
		StartedAt:         now,
		Deadline:          now.Add(time.Duration(challenge.TimeLimitMinutes) * time.Minute),
	}, nil
}

// Expired reports whether the deadline has been reached.
func (a ChallengeAttempt) Expired(now time.Time) bool {
	return !now.Before(a.Deadline)
}
