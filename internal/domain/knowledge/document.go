package knowledge

import (
	"context"
	"errors"
)

// ErrDocumentNotFound is returned by DocumentRepository.Get when itemID has
// no stored document text — either it was never persisted (a chunk imported
// before spec 2.18) or it was removed (RemoveSource). See
// specs/phases/phase-02-knowledge-engine/18-notebooklm-style-citations.md.
var ErrDocumentNotFound = errors.New("knowledge document not found")

// DocumentRepository persists a document's full imported text, kept apart
// from Item so an ordinary item read never drags it along. Today the only
// implementation is SQLite-backed (internal/infrastructure/sqlite).
type DocumentRepository interface {
	// Save inserts content for itemID, or replaces it if a document is
	// already stored for that item.
	Save(ctx context.Context, itemID, sessionID, content string) error
	// Get returns itemID's stored text within sessionID, or
	// ErrDocumentNotFound if none is stored.
	Get(ctx context.Context, sessionID, itemID string) (string, error)
	// DeleteByItemID removes itemID's stored text within sessionID. It is a
	// no-op, not an error, when no document is stored.
	DeleteByItemID(ctx context.Context, sessionID, itemID string) error
}

// SourceDocument is the source viewer's read model: a document's full text,
// already cut into ordered Segments at its own chunks' boundaries, so the
// client highlights a cited passage without doing any offset arithmetic of
// its own (a Go byte offset means nothing to JavaScript's UTF-16 strings).
type SourceDocument struct {
	ItemID string
	// Title is the document's H1, falling back to its file name — the same
	// value as SessionSource.Title.
	Title string
	// Path is the stable, root-relative display path. Never the absolute
	// source path.
	Path     string
	Segments []DocumentSegment
}

// DocumentSegment is one ordered slice of a SourceDocument's text: either a
// chunk's own passage (ChunkID set) or the trimmed whitespace gap between
// two chunks (ChunkID empty). Segments reassemble to the document's stored
// text exactly.
type DocumentSegment struct {
	Text    string
	ChunkID string
}
