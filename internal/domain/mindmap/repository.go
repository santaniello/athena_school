package mindmap

import (
	"context"
	"errors"
	"time"
)

// ErrMindMapNotFound is returned when no mind map matches the given ID.
var ErrMindMapNotFound = errors.New("mind map not found")

// ErrSessionNotFound is returned when a mind map is stored for a session that
// does not exist, for example one deleted while the map was being generated.
var ErrSessionNotFound = errors.New("mind map session not found")

// Summary is a mind map as the catalog lists it, without its tree and diagram.
type Summary struct {
	ID           string
	SessionID    string
	SessionTitle string
	Title        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MessageID    string // empty unless an offer in the study chat produced the map
}

// Repository stores mind maps.
type Repository interface {
	// Create stores the mind map, or returns ErrSessionNotFound when its
	// session does not exist.
	Create(ctx context.Context, mindMap MindMap) error
	// Get returns the mind map with its tree and diagram, or ErrMindMapNotFound.
	Get(ctx context.Context, id string) (MindMap, error)
	// List returns every mind map, newest first.
	List(ctx context.Context) ([]Summary, error)
	// FindByMessage returns the mind map an offer produced under the chat
	// message, or ErrMindMapNotFound.
	FindByMessage(ctx context.Context, messageID string) (Summary, error)
	// ListBySession returns the mind maps an offer produced in the session,
	// oldest first.
	ListBySession(ctx context.Context, sessionID string) ([]Summary, error)
	// Update replaces the session title snapshot, title, tree, diagram and
	// update time of the mind map with that ID, or returns ErrMindMapNotFound.
	// Its session, message and creation time stay.
	Update(ctx context.Context, mindMap MindMap) error
	// Delete removes the mind map, or returns ErrMindMapNotFound.
	Delete(ctx context.Context, id string) error
}
