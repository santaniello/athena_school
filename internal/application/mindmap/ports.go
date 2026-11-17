package mindmap

import (
	"context"
	"errors"

	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

// ErrSourceSessionNotFound is returned when the session a mind map is asked
// for does not exist, or no session is given. SessionReader implementations
// return it for an unknown session.
var ErrSourceSessionNotFound = errors.New("source session not found")

// SessionReader reads the topic and goal of a study session.
type SessionReader interface {
	// ReadSession returns the session's topic and goal, or
	// ErrSourceSessionNotFound if the session does not exist.
	ReadSession(ctx context.Context, sessionID string) (domainmindmap.SourceSession, error)
}

// SummaryReader reads a session's conversation summary.
type SummaryReader interface {
	// ReadSummary returns the summary text, or "" when the session has none.
	ReadSummary(ctx context.Context, sessionID string) (string, error)
}

// ProfileReader reads the user's profile.
type ProfileReader interface {
	ReadProfile(ctx context.Context) (domainmindmap.GenerationProfile, error)
}

// NotesSearcher searches the notes attached to a session. It fails when the
// notes index is not ready; generation then continues without notes.
type NotesSearcher interface {
	SearchNotes(ctx context.Context, sessionID, query string) ([]domainmindmap.NoteExcerpt, error)
}
