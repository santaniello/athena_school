// Package knowledge implements the use cases for retrieving a study
// session's knowledge and keeping its search index consistent.
package knowledge

import (
	"context"
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// Transactor runs fn inside a single atomic unit of work.
// DeleteSessionsWithKnowledge reads the chunk IDs and deletes the sessions
// inside one, so a failure leaves neither half done — see db.go's
// single-connection pool, which turns "one transaction open" into "every
// other call blocks until it's done". Defined here (consumer side) per Go
// convention; implemented by internal/infrastructure/sqlite.SQLTransactor.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// IndexGuard reports whether a knowledge mutation may proceed right now, and
// reserves the index against a concurrent reload for as long as one is in
// flight. Defined here (consumer side) per Go convention; implemented by
// *IndexLoader.
//
// BeginMutation/EndMutation hold the reservation for a mutation's entire
// duration — its transaction commit through its post-commit VectorStore
// reconciliation — so a reload started partway through can never publish a
// snapshot older than what the mutation just wrote, or race its Remove.
// Every mutation that touches the VectorStore must wrap its full body in
// them.
//
// Status reports the index coordinator's current lifecycle snapshot;
// Retrieve reads it to distinguish a valid empty index from one that has
// never loaded (see specs/phases/phase-02-knowledge-engine/05-rag-integration.md).
type IndexGuard interface {
	BeginMutation() error
	EndMutation()
	Status() domainknowledge.IndexStatus
}

// Service implements knowledge retrieval and session-knowledge cleanup
// against the application's ports.
type Service struct {
	items      domainknowledge.Repository
	llm        domainllm.Provider
	chunks     domainknowledge.ChunkRepository
	tx         Transactor
	store      domainknowledge.VectorStore
	index      IndexGuard
	thresholds domainknowledge.RetrievalThresholds
}

// NewService creates the knowledge service.
func NewService(
	items domainknowledge.Repository,
	llm domainllm.Provider,
	chunks domainknowledge.ChunkRepository,
	tx Transactor,
	store domainknowledge.VectorStore,
	index IndexGuard,
	thresholds domainknowledge.RetrievalThresholds,
) *Service {
	return &Service{
		items: items, llm: llm, chunks: chunks, tx: tx,
		store: store, index: index, thresholds: thresholds,
	}
}

// reconcileContext returns a short-lived context for post-commit VectorStore
// reconciliation (Add/Remove), independent of the original request context.
// It is deliberately not the caller's ctx: a request context canceled right
// after commit must not skip mandatory in-memory cleanup and leave stale
// content searchable.
func reconcileContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
