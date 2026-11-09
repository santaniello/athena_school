package exercise

import (
	"errors"
	"fmt"
)

var (
	// ErrChallengeHasNoRubric is returned when an attempt is started on a
	// challenge that has no rubric yet.
	ErrChallengeHasNoRubric = errors.New("challenge has no rubric")
	// ErrAttemptInProgress is returned when another attempt is already running.
	// The concrete error is an *AttemptInProgressError carrying its id.
	ErrAttemptInProgress = errors.New("an attempt is already in progress")
	// ErrSubmissionTooLate is returned when a submission arrives after the
	// deadline plus the grace period. The attempt is discarded.
	ErrSubmissionTooLate = errors.New("submission arrived too late")
	// ErrAttemptFinished is returned when a finished attempt is discarded.
	ErrAttemptFinished = errors.New("a finished attempt cannot be discarded")
	// ErrAttemptNotFinished is returned when the result of an attempt that is
	// not FINISHED is asked for.
	ErrAttemptNotFinished = errors.New("the attempt is not finished")
)

// AttemptInProgressError is the concrete ErrAttemptInProgress: it names the
// running attempt so the UI can offer to discard it.
type AttemptInProgressError struct {
	AttemptID string
}

func (e *AttemptInProgressError) Error() string {
	return fmt.Sprintf("%s: %s", ErrAttemptInProgress, e.AttemptID)
}

// Is makes errors.Is(err, ErrAttemptInProgress) true.
func (e *AttemptInProgressError) Is(target error) bool {
	return target == ErrAttemptInProgress
}
