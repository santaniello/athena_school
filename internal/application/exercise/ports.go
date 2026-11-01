// Package exercise holds the exercise use cases. Today that is the
// challenge catalog: creating a challenge from a studied session, reading
// one and listing them. See
// specs/phases/phase-03-architecture-lab/01-challenge-domain-and-persistence.md.
package exercise

import (
	"context"
	"errors"

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
