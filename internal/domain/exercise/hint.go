package exercise

import (
	"context"
	"errors"
	"time"
)

// MaxHints is how many hints the tutor gives during one attempt.
const MaxHints = 3

var (
	// ErrNoHintsLeft is returned when MaxHints hints were already given for the attempt.
	ErrNoHintsLeft = errors.New("no hints left for this attempt")
	// ErrHintInProgress is returned when a hint is already being requested for
	// the attempt, or when a concurrent request took the position first.
	ErrHintInProgress = errors.New("a hint is already being requested for this attempt")
)

// Hint is one nudge the tutor gave during an attempt: a direction or a
// question, never a solution. See
// specs/phases/phase-03-architecture-lab/10-01-tutor-hints.md.
type Hint struct {
	ID        string
	AttemptID string
	Position  int // 1..MaxHints, in the order given
	Content   string
	CreatedAt time.Time
}

// HintRepository persists Hints. A stored hint is immutable, so there is
// deliberately no Update and no Delete: hints disappear with their unfinished
// attempt.
type HintRepository interface {
	// Add stores the hint. It returns ErrNoHintsLeft when the attempt already
	// has MaxHints, even under concurrent calls, and ErrHintInProgress when
	// another call stored the same position first.
	Add(ctx context.Context, hint Hint) error
	// ListByAttempt returns the attempt's hints in the order given.
	ListByAttempt(ctx context.Context, attemptID string) ([]Hint, error)
}

// HintInput is everything a HintGenerator may look at.
type HintInput struct {
	Challenge Challenge
	Rubric    RubricVersion
	BoardXML  string // the current board; may be empty
	Previous  []Hint // the hints already given, in order
}

// HintGenerator writes the next hint. It is implemented per lab, so the
// generic engine knows nothing about diagrams.
type HintGenerator interface {
	GenerateHint(ctx context.Context, input HintInput) (string, error)
}
