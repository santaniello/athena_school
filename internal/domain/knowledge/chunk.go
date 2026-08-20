package knowledge

import (
	"context"
	"time"
)

// Chunk is a fidelity-preserving slice of a knowledge source's raw text,
// carrying its own embedding for retrieval. See
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md.
type Chunk struct {
	ID     string
	Source string // athena | user_note | imported_doc
	Topic  string
	Status string
	// ItemID is the owning knowledge Item: the extracted Item for
	// Source == athena, the shadow Item for Source == imported_doc. Always set.
	ItemID         string
	FilePath       string
	Heading        string
	Content        string
	Embedding      []float32
	EmbeddingModel string
	// ItemUpdatedAt is zero for imported files; it detects stale
	// Knowledge Item chunks after an indexing failure. Imported-file
	// chunks deliberately stay zero here even though they carry an
	// ItemID — their dedup/staleness signal is IngestedFile.MTime, a
	// different mechanism serving a different purpose.
	ItemUpdatedAt time.Time
	CreatedAt     time.Time
}

// ChunkLoadResult is ListCurrent's report: the chunks safe to index, plus
// every row it excluded and why. A row is excluded rather than failing the
// whole load so one corrupt/stale chunk never makes the rest unavailable.
type ChunkLoadResult struct {
	Chunks []Chunk
	Issues []ChunkLoadIssue
}

// ChunkRepository persists Chunks. Today the only implementation is
// SQLite-backed (internal/infrastructure/sqlite).
type ChunkRepository interface {
	SaveAll(ctx context.Context, chunks []Chunk) error
	// ListAll returns every persisted chunk regardless of freshness —
	// startup must use ListCurrent instead; this remains for callers that
	// genuinely want the raw table (see chunk_repository_test.go).
	ListAll(ctx context.Context) ([]Chunk, error)
	// ListCurrent returns only chunks safe to index: embeddingModel matches,
	// the owning knowledge_items row exists with matching source/topic/
	// status, and (for every source except imported_doc, whose freshness is
	// governed by ingested_files instead) ItemUpdatedAt equals the Item's
	// current UpdatedAt. Excluded rows are reported as Issues, never
	// silently dropped, except a wrong-embedding-model row, which is
	// expected reindex work rather than a corruption warning. A query,
	// scan, or iteration failure returns an error for the entire load
	// instead of reporting an empty result.
	ListCurrent(ctx context.Context, embeddingModel string) (ChunkLoadResult, error)
	// DeleteByFilePath removes every chunk previously produced by path and
	// returns the IDs removed, so a caller can evict them from an
	// in-memory index after this call's transaction commits.
	DeleteByFilePath(ctx context.Context, path string) ([]string, error)
	// DeleteByItemID removes every chunk owned by itemID and returns the
	// IDs removed, so a caller can evict them from an in-memory index
	// after this call's transaction commits.
	DeleteByItemID(ctx context.Context, itemID string) ([]string, error)
	// UpdateMetadataByItemID overwrites topic/status on every chunk owned
	// by itemID and returns the updated rows, so a caller can upsert them
	// into an in-memory index without a new embedding call.
	UpdateMetadataByItemID(ctx context.Context, itemID, topic, status string) ([]Chunk, error)
}

// IngestedFile records the dedup state for one previously imported file.
type IngestedFile struct {
	Path           string
	MTime          int64
	EmbeddingModel string
	ChunkCount     int
	// ItemID is the shadow Item's stable ID, carried across re-imports.
	ItemID string
}

// IngestedFileRepository persists IngestedFile dedup records. Today the
// only implementation is SQLite-backed (internal/infrastructure/sqlite).
type IngestedFileRepository interface {
	// ListAll returns every ingested file, keyed by path — one query per import.
	ListAll(ctx context.Context) (map[string]IngestedFile, error)
	Upsert(ctx context.Context, file IngestedFile) error
}
