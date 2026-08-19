package ingest

import (
	"context"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// Transactor runs fn inside a single atomic unit of work, so the chunk/
// item/ingested-file replace in ImportFolder either all lands or none of
// it does. Defined here (consumer side) per Go convention; implemented by
// internal/infrastructure/sqlite.SQLTransactor.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Service implements the notes-import pipeline against the application's
// ports. There is no VectorStore dependency here — that is introduced by
// later specs; this pipeline's job ends at persisting to SQLite.
type Service struct {
	chunks        domainknowledge.ChunkRepository
	ingestedFiles domainknowledge.IngestedFileRepository
	items         domainknowledge.Repository
	llm           domainllm.Provider
	tx            Transactor
}

// NewService creates a notes-import Service.
func NewService(
	chunks domainknowledge.ChunkRepository,
	ingestedFiles domainknowledge.IngestedFileRepository,
	items domainknowledge.Repository,
	llm domainllm.Provider,
	tx Transactor,
) *Service {
	return &Service{chunks: chunks, ingestedFiles: ingestedFiles, items: items, llm: llm, tx: tx}
}
