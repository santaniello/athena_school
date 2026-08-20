package knowledge

import (
	"context"
	"errors"
	"math"
	"strings"
)

// SearchFilters narrows a vector Search. An empty field means no constraint
// on it — exact Go string equality, never case-folded or trimmed.
type SearchFilters struct{ Topic, Source, Status string }

// ScoredChunk pairs a Chunk with its cosine similarity to a Search query.
type ScoredChunk struct {
	Chunk Chunk
	Score float32
}

// VectorStore is an in-process index over current Chunks, kept coherent
// with SQLite by explicit Add/ReplaceAll/Remove calls from the application
// layer. Implemented by internal/infrastructure/vectorstore.
type VectorStore interface {
	// Add validates and normalizes the batch, then upserts it by Chunk.ID:
	// an existing ID is replaced in place, a new one is appended.
	Add(ctx context.Context, chunks []Chunk) error
	// ReplaceAll validates and normalizes the batch, then atomically
	// replaces the entire active snapshot with it — including marking the
	// store ready when the batch is empty.
	ReplaceAll(ctx context.Context, chunks []Chunk) error
	// Search returns at most topK chunks matching filters, ordered by
	// cosine similarity descending, then Chunk.ID ascending.
	Search(ctx context.Context, query []float32, topK int, filters SearchFilters) ([]ScoredChunk, error)
	// Remove evicts every given ID. Unknown IDs are silently ignored.
	Remove(ctx context.Context, ids []string) error
	// Len reports how many chunks are currently indexed.
	Len() int
}

// Typed sentinels for VectorStore so callers use errors.Is instead of
// depending on error text.
var (
	ErrInvalidTopK            = errors.New("topK must be greater than zero")
	ErrInvalidVector          = errors.New("vector must be non-empty, finite, and have non-zero norm")
	ErrInvalidChunkID         = errors.New("chunk id is required")
	ErrDuplicateChunkID       = errors.New("chunk ids must be unique within a batch")
	ErrUnknownSource          = errors.New("unknown knowledge source")
	ErrVectorStoreUnavailable = errors.New("vector store is unavailable")
)

// ValidateChunk reports whether chunk is valid for indexing: a non-blank ID
// (never trimmed or rewritten), a known Source, a known Status, and a valid
// embedding (see ValidateVector). Structural errors take precedence over the
// vector error, matching VectorStore's documented error precedence.
func ValidateChunk(chunk Chunk) error {
	if strings.TrimSpace(chunk.ID) == "" {
		return ErrInvalidChunkID
	}
	switch chunk.Source {
	case SourceAthena, SourceUserNote, SourceImportedDoc:
	default:
		return ErrUnknownSource
	}
	switch chunk.Status {
	case StatusDraft, StatusApproved, StatusDeprecated:
	default:
		return ErrUnknownStatus
	}
	return ValidateVector(chunk.Embedding)
}

// ValidateVector reports whether vec is usable for indexing or search: non-
// empty, every component finite (no NaN/±Inf), and a non-zero Euclidean
// norm. The norm accumulates in float64 so a high-dimensional vector of
// large components cannot overflow the float32 embedding itself carries.
func ValidateVector(vec []float32) error {
	if len(vec) == 0 {
		return ErrInvalidVector
	}
	var sumSquares float64
	for _, v := range vec {
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return ErrInvalidVector
		}
		sumSquares += f * f
	}
	if sumSquares == 0 {
		return ErrInvalidVector
	}
	return nil
}

// ChunkLoadIssue records why one persisted chunk was excluded from a
// ListCurrent load or a ReplaceAll snapshot. Only safe fields are carried —
// Reason is a stable code the UI maps to English copy, never a raw
// technical error.
type ChunkLoadIssue struct {
	ChunkID  string
	ItemID   string
	Source   string
	FilePath string
	Reason   string
}

// IndexState is the externally observable lifecycle state of the vector
// index coordinator.
type IndexState string

// The four IndexState values a coordinator can be in.
const (
	IndexStateLoading           IndexState = "loading"
	IndexStateReady             IndexState = "ready"
	IndexStateReadyWithWarnings IndexState = "ready_with_warnings"
	IndexStateFailed            IndexState = "failed"
)

// IndexStatus is the coordinator's current lifecycle snapshot. HasSnapshot
// distinguishes an initial load from a retry that can keep serving a
// previously published, still-valid snapshot.
type IndexStatus struct {
	State       IndexState
	HasSnapshot bool
	Issues      []ChunkLoadIssue
	LastError   string
}
