package exercise

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
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
	// The evaluation states of spec 3.5: SUBMITTED → EVALUATING → EVALUATED,
	// and EVALUATING → EVALUATION_FAILED → EVALUATING for a manual retry.
	AttemptEvaluating       AttemptState = "EVALUATING"
	AttemptEvaluated        AttemptState = "EVALUATED"
	AttemptEvaluationFailed AttemptState = "EVALUATION_FAILED"
	// AttemptFinished is the end of an attempt's lifecycle, once it has been
	// scored. Nothing reaches it yet; it is named here because a finished
	// attempt is never discarded. Spec 3.12 adds the transitions into it.
	AttemptFinished AttemptState = "FINISHED"
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
	FailureReason     string // why the last evaluation failed; kept until the next retry
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

var (
	// ErrAttemptNotEvaluable is returned when an evaluation is started on an
	// attempt that is neither SUBMITTED nor EVALUATION_FAILED, including one
	// already being evaluated.
	ErrAttemptNotEvaluable = errors.New("attempt cannot be evaluated now")
	// ErrAttemptNotEvaluating is returned when an evaluation is completed or
	// failed on an attempt that is not EVALUATING.
	ErrAttemptNotEvaluating = errors.New("attempt is not being evaluated")
)

// StartEvaluation moves a SUBMITTED or EVALUATION_FAILED attempt to
// EVALUATING and clears the previous failure reason. The receiver is not
// modified.
func (a ChallengeAttempt) StartEvaluation() (ChallengeAttempt, error) {
	if a.State != AttemptSubmitted && a.State != AttemptEvaluationFailed {
		return ChallengeAttempt{}, ErrAttemptNotEvaluable
	}
	a.State = AttemptEvaluating
	a.FailureReason = ""
	return a, nil
}

// CompleteEvaluation moves an EVALUATING attempt to EVALUATED.
func (a ChallengeAttempt) CompleteEvaluation() (ChallengeAttempt, error) {
	if a.State != AttemptEvaluating {
		return ChallengeAttempt{}, ErrAttemptNotEvaluating
	}
	a.State = AttemptEvaluated
	return a, nil
}

// FailEvaluation moves an EVALUATING attempt to EVALUATION_FAILED, recording
// the reason until the next retry.
func (a ChallengeAttempt) FailEvaluation(reason string) (ChallengeAttempt, error) {
	if a.State != AttemptEvaluating {
		return ChallengeAttempt{}, ErrAttemptNotEvaluating
	}
	a.State = AttemptEvaluationFailed
	a.FailureReason = reason
	return a, nil
}

// MaxSubmissionContentBytes bounds a submission's content (2 MiB).
const MaxSubmissionContentBytes = 2 << 20

var (
	// ErrAttemptNotInProgress is returned when an attempt that is not IN_PROGRESS is submitted.
	ErrAttemptNotInProgress = errors.New("attempt is not in progress")
	// ErrSubmissionContentRequired is returned when the submitted content is empty.
	ErrSubmissionContentRequired = errors.New("submission content is required")
	// ErrSubmissionContentTypeRequired is returned when the content type is blank.
	ErrSubmissionContentTypeRequired = errors.New("submission content type is required")
	// ErrSubmissionTooLarge is returned when the content exceeds MaxSubmissionContentBytes.
	ErrSubmissionTooLarge = errors.New("submission content is too large")
	// ErrSubmitReasonInvalid is returned for an unknown submit reason.
	ErrSubmitReasonInvalid = errors.New("submit reason is invalid")
)

// Submission is the immutable artifact an attempt ended with. Content is an
// opaque payload: exercise never parses it.
type Submission struct {
	ID          string
	AttemptID   string
	ContentType string
	Content     string
	Checksum    string // hex SHA-256 of Content
	SubmittedAt time.Time
}

// Submit ends an IN_PROGRESS attempt and returns it as SUBMITTED together with
// its Submission. The receiver is not modified; the caller assigns the
// submission's ID.
func (a ChallengeAttempt) Submit(contentType, content string, reason SubmitReason, now time.Time) (ChallengeAttempt, Submission, error) {
	if a.State != AttemptInProgress {
		return ChallengeAttempt{}, Submission{}, ErrAttemptNotInProgress
	}
	if err := validateSubmission(contentType, content, reason); err != nil {
		return ChallengeAttempt{}, Submission{}, err
	}
	checksum := sha256.Sum256([]byte(content))
	submission := Submission{
		AttemptID:   a.ID,
		ContentType: contentType,
		Content:     content,
		Checksum:    hex.EncodeToString(checksum[:]),
		SubmittedAt: now,
	}
	a.State = AttemptSubmitted
	a.SubmittedAt = now
	a.SubmitReason = reason
	return a, submission, nil
}

func validateSubmission(contentType, content string, reason SubmitReason) error {
	switch {
	case content == "":
		return ErrSubmissionContentRequired
	case strings.TrimSpace(contentType) == "":
		return ErrSubmissionContentTypeRequired
	case len(content) > MaxSubmissionContentBytes:
		return ErrSubmissionTooLarge
	case reason != SubmitReasonUser && reason != SubmitReasonTimeExpired:
		return ErrSubmitReasonInvalid
	}
	return nil
}

// SessionExercise links one study session to one challenge. It is created when
// the user accepts the tutor's proposal, never when it is proposed.
type SessionExercise struct {
	ID          string
	SessionID   string
	ChallengeID string
	MessageID   string // the assistant message the chat card hangs under
	CreatedAt   time.Time
}
