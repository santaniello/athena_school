// Package study holds the Study Mode use cases: starting a session,
// exchanging messages with the LLM, and ending a session. See
// specs/phases/phase-01-desktop-mvp/06-study-mode.md.
package study

import (
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Service implements the Study Mode use cases against a
// domainstudy.SessionRepository, a domainstudy.MessageRepository, a
// domainllm.Provider and a domainprofile.Store.
type Service struct {
	sessions domainstudy.SessionRepository
	messages domainstudy.MessageRepository
	llm      domainllm.Provider
	profiles domainprofile.Store
}

// NewService creates a Service backed by the given ports.
func NewService(
	sessions domainstudy.SessionRepository,
	messages domainstudy.MessageRepository,
	llm domainllm.Provider,
	profiles domainprofile.Store,
) *Service {
	return &Service{sessions: sessions, messages: messages, llm: llm, profiles: profiles}
}
