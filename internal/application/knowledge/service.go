// Package knowledge implements use cases for extracting and saving knowledge.
package knowledge

import (
	"context"

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

// Service implements knowledge extraction and Explorer management against
// the application's ports.
type Service struct {
	items    domainknowledge.Repository
	sessions domainstudy.SessionRepository
	messages domainstudy.MessageRepository
	llm      domainllm.Provider
	configs  domainconfig.Store
	chunks   domainknowledge.ChunkRepository
	tx       Transactor
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
) *Service {
	return &Service{
		items: items, sessions: sessions, messages: messages,
		llm: llm, configs: configs, chunks: chunks, tx: tx,
	}
}
