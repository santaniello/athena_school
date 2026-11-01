package exercise

import (
	"context"
	"errors"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// ErrInvalidGeneration is returned when the model's output is not a valid
// challenge with a valid rubric. Nothing is stored.
var ErrInvalidGeneration = errors.New("generated challenge is invalid")

// SummaryReader reads a session's conversation summary. Defined here
// (consumer side) so this package never imports the study domain.
type SummaryReader interface {
	// ReadSummary returns the summary text, or "" when the session has none.
	ReadSummary(ctx context.Context, sessionID string) (string, error)
}

// ProfileReader reads the user's profile.
type ProfileReader interface {
	ReadProfile(ctx context.Context) (domainexercise.GenerationProfile, error)
}

// NotesSearcher searches the notes attached to a session. It fails when the
// notes index is not ready; generation then continues without notes.
type NotesSearcher interface {
	SearchNotes(ctx context.Context, sessionID, query string) ([]domainexercise.NoteExcerpt, error)
}
