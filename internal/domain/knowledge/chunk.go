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

// ChunkRepository persists Chunks. Today the only implementation is
// SQLite-backed (internal/infrastructure/sqlite).
type ChunkRepository interface {
	SaveAll(ctx context.Context, chunks []Chunk) error
	ListAll(ctx context.Context) ([]Chunk, error)
	DeleteByFilePath(ctx context.Context, path string) error
	DeleteByItemID(ctx context.Context, itemID string) error
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
