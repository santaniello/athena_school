// Package exercise holds the exercise use cases. Today that is the
// challenge catalog: creating a challenge from a studied session, reading
// one and listing them. See
// specs/phases/phase-03-architecture-lab/01-challenge-domain-and-persistence.md.
package exercise

import (
	"context"
	"errors"
	"time"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// ErrSourceSessionNotFound is returned by Create when the source session does
// not exist, or no source session is given. SessionTopicReader implementations
// return it for an unknown session.
var ErrSourceSessionNotFound = errors.New("source session not found")

// SessionTopicReader reads the topic and goal of a study session. It is
// defined here (consumer side) so this package never imports the study
// domain; the composition root adapts the study session repository to it.
type SessionTopicReader interface {
	// ReadSession returns the session's topic and goal, or
	// ErrSourceSessionNotFound if the session does not exist.
	ReadSession(ctx context.Context, sessionID string) (domainexercise.SourceSession, error)
}

// Clock tells the current time. The attempt service reads time only through
// it, so the deadline and grace-period rules are testable without sleeping.
type Clock interface {
	Now() time.Time
}

// SessionExists reports whether a study session exists. It is defined here
// (consumer side) so this package never imports the study domain; the
// composition root adapts the study session repository to it.
type SessionExists interface {
	// Exists returns false, with no error, for an unknown session.
	Exists(ctx context.Context, sessionID string) (bool, error)
}

// Transactor runs fn inside a single atomic unit of work, so discarding a
// running attempt and starting the next one either both land or neither
// does. Implemented by internal/infrastructure/sqlite.SQLTransactor.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// PassScoreProvider supplies the minimum passing score in force. Defined here
// (consumer side); implemented by the settings service. It returns an error
// rather than a default when the setting cannot be read, so a threshold that
// was not verified is never frozen on an attempt.
type PassScoreProvider interface {
	Current(ctx context.Context) (int, error)
}

// ChallengeGenerator writes a challenge from a studied session. Defined here
// (consumer side); implemented by *GenerationService.
type ChallengeGenerator interface {
	GenerateChallenge(ctx context.Context, sessionID string, difficulty domainexercise.Difficulty) (domainexercise.Challenge, error)
}
