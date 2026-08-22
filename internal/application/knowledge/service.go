// Package knowledge implements use cases for extracting and saving knowledge.
package knowledge

import (
	"context"
	"time"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Transactor runs fn inside a single atomic unit of work. Approve,
// Deprecate, UpdateItem and DeleteItem each read an Item then write it
// back (or write its chunks); wrapping that in one transaction closes the
// read-write gap a concurrent call could otherwise land in — see db.go's
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
// CheckMutationAllowed is a point-in-time read, kept for callers that only
// need an early rejection. BeginMutation/EndMutation instead hold the
// reservation for a mutation's entire duration — its transaction commit
// through its post-commit VectorStore reconciliation — so a reload started
// partway through can never publish a snapshot older than what the mutation
// just wrote, or race its Add/Remove. Every mutation that touches the
// VectorStore must wrap its full body in BeginMutation/EndMutation, not
// just check CheckMutationAllowed once up front.
//
// Status reports the index coordinator's current lifecycle snapshot;
// Retrieve reads it to distinguish a valid empty index from one that has
// never loaded (see specs/phases/phase-02-knowledge-engine/05-rag-integration.md).
type IndexGuard interface {
	CheckMutationAllowed() error
	BeginMutation() error
	EndMutation()
	Status() domainknowledge.IndexStatus
}

// Service implements knowledge extraction and Explorer management against
// the application's ports.
type Service struct {
	items      domainknowledge.Repository
	sessions   domainstudy.SessionRepository
	messages   domainstudy.MessageRepository
	llm        domainllm.Provider
	configs    domainconfig.Store
	chunks     domainknowledge.ChunkRepository
	tx         Transactor
	store      domainknowledge.VectorStore
	index      IndexGuard
	thresholds domainknowledge.RetrievalThresholds
}

// NewService creates a knowledge extraction and Explorer management service.
func NewService(
	items domainknowledge.Repository,
	sessions domainstudy.SessionRepository,
	messages domainstudy.MessageRepository,
	llm domainllm.Provider,
	configs domainconfig.Store,
	chunks domainknowledge.ChunkRepository,
	tx Transactor,
	store domainknowledge.VectorStore,
	index IndexGuard,
	thresholds domainknowledge.RetrievalThresholds,
) *Service {
	return &Service{
		items: items, sessions: sessions, messages: messages,
		llm: llm, configs: configs, chunks: chunks, tx: tx,
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
