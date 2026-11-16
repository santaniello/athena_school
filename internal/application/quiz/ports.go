// Package quiz holds the Interactive Quiz use cases: generating a quiz from a
// study session, submitting an attempt and reading the catalog and history. It
// never imports the exercise or study packages: everything it reads from them
// comes through the small ports below, which the composition root adapts. See
// specs/decisions/ADR-007-interactive-quiz.md and
// specs/phases/phase-03-architecture-lab/21-interactive-quiz-core.md.
package quiz

import (
	"context"
	"errors"

	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

var (
	// ErrSourceSessionNotFound is returned when the session a quiz is asked
	// for does not exist, or no session is given. SessionReader
	// implementations return it for an unknown session.
	ErrSourceSessionNotFound = errors.New("source session not found")
	// ErrInvalidGeneration is returned when the model's output is not a valid
	// quiz. Nothing is stored.
	ErrInvalidGeneration = errors.New("generated quiz is invalid")
)

// SessionReader reads the topic and goal of a study session.
type SessionReader interface {
	// ReadSession returns the session's topic and goal, or
	// ErrSourceSessionNotFound if the session does not exist.
	ReadSession(ctx context.Context, sessionID string) (domainquiz.SourceSession, error)
}

// SummaryReader reads a session's conversation summary.
type SummaryReader interface {
	// ReadSummary returns the summary text, or "" when the session has none.
	ReadSummary(ctx context.Context, sessionID string) (string, error)
}

// ProfileReader reads the user's profile.
type ProfileReader interface {
	ReadProfile(ctx context.Context) (domainquiz.GenerationProfile, error)
}

// NotesSearcher searches the notes attached to a session. It fails when the
// notes index is not ready; generation then continues without notes.
type NotesSearcher interface {
	SearchNotes(ctx context.Context, sessionID, query string) ([]domainquiz.NoteExcerpt, error)
}

// Generator writes a quiz from a session and ties it to the study-chat
// message whose offer asked for it. GenerationService implements it; the offer
// service depends on this port so it can be tested without a model.
type Generator interface {
	// GenerateForMessage generates and stores a quiz for the session, linked to
	// the message. A failure stores nothing.
	GenerateForMessage(ctx context.Context, sessionID, messageID string, spec domainquiz.Spec) (domainquiz.Quiz, error)
}
