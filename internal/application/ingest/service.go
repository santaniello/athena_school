package ingest

import (
	"context"
	"time"

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

// IndexGuard reports whether a knowledge mutation may proceed right now, and
// reserves the index against a concurrent reload for as long as one is in
// flight. Defined here (consumer side) per Go convention; implemented by
// *applicationknowledge.IndexLoader.
//
// CheckMutationAllowed is a point-in-time read. ImportFolder instead holds
// BeginMutation/EndMutation for its entire walk — a point-in-time check
// alone would let a retry started mid-import interleave its
// ListCurrent/ReplaceAll with individual files' transaction commits and
// VectorStore reconciliation.
type IndexGuard interface {
	CheckMutationAllowed() error
	BeginMutation() error
	EndMutation()
}

// Service implements the notes-import pipeline against the application's
// ports.
type Service struct {
	chunks        domainknowledge.ChunkRepository
	ingestedFiles domainknowledge.IngestedFileRepository
	items         domainknowledge.Repository
	llm           domainllm.Provider
	tx            Transactor
	store         domainknowledge.VectorStore
	index         IndexGuard
}

// NewService creates a notes-import Service.
func NewService(
	chunks domainknowledge.ChunkRepository,
	ingestedFiles domainknowledge.IngestedFileRepository,
	items domainknowledge.Repository,
	llm domainllm.Provider,
	tx Transactor,
	store domainknowledge.VectorStore,
	index IndexGuard,
) *Service {
	return &Service{
		chunks: chunks, ingestedFiles: ingestedFiles, items: items, llm: llm, tx: tx,
		store: store, index: index,
	}
}

// IndexingWarning wraps a post-commit VectorStore reconciliation failure —
// ImportFolder's Remove(old chunk IDs)/Add(new chunks) calls after its
// SQLite transaction has already committed. The durable import is never
// rolled back for this; ImportFolder reports it as an ingest.FileFailure
// under Summary.IndexWarnings rather than Summary.Failures, since
// ingested_files now legitimately records the new mtime/model and a
// repeated import would correctly skip the file.
type IndexingWarning struct {
	// Err is the underlying VectorStore error (Add/Remove).
	Err error
}

func (w *IndexingWarning) Error() string {
	return "knowledge index reconciliation failed: " + w.Err.Error()
}

func (w *IndexingWarning) Unwrap() error {
	return w.Err
}

// reconcileContext returns a short-lived context for post-commit VectorStore
// reconciliation (Add/Remove), independent of the original request context.
// It is deliberately not the caller's ctx: a request context canceled right
// after commit must not skip mandatory in-memory cleanup and leave stale
// content searchable.
func reconcileContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
