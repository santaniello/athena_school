// Package study holds the Study Mode use cases: starting a session,
// exchanging messages with the LLM, and ending a session. See
// specs/phases/phase-01-desktop-mvp/06-study-mode.md.
package study

import (
	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Service implements the Study Mode use cases against a
// domainstudy.SessionRepository, a domainstudy.MessageRepository, a
// domainllm.Provider, a domainprofile.Store, a domainfolder.Repository
// (used to fall back to the default folder and validate a chosen one
// exists before creating a session), a domainknowledge.Retriever (used by
// every SendMessage call), a Transactor (atomic message + ContextUsage
// writes), a
// domainllm.ModelContextResolver (resolves a stream's model to its context
// window; see specs/phases/phase-02-knowledge-engine/06-study-context-limits.md),
// and a domainknowledge.MessageSourceRepository (persists the Sources
// behind a completed assistant message, so they survive a resume — see
// specs/phases/phase-02-knowledge-engine/09-persistent-provenance.md).
// domain/study never imports domain/knowledge itself; this is the layer
// that composes the two, both here and in Resume's MessageWithSources.
type Service struct {
	sessions       domainstudy.SessionRepository
	messages       domainstudy.MessageRepository
	llm            domainllm.Provider
	profiles       domainprofile.Store
	folders        domainfolder.Repository
	retriever      domainknowledge.Retriever
	tx             Transactor
	catalog        domainllm.ModelContextResolver
	messageSources domainknowledge.MessageSourceRepository
	inFlight       *inFlightCoordinator
}

// NewService creates a Service backed by the given ports.
func NewService(
	sessions domainstudy.SessionRepository,
	messages domainstudy.MessageRepository,
	llm domainllm.Provider,
	profiles domainprofile.Store,
	folders domainfolder.Repository,
	retriever domainknowledge.Retriever,
	tx Transactor,
	catalog domainllm.ModelContextResolver,
	messageSources domainknowledge.MessageSourceRepository,
) *Service {
	return &Service{
		sessions: sessions, messages: messages, llm: llm,
		profiles: profiles, folders: folders, retriever: retriever,
		tx: tx, catalog: catalog, messageSources: messageSources,
		inFlight: newInFlightCoordinator(),
	}
}
