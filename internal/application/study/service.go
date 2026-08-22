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
// exists before creating a session), and a domainknowledge.Retriever (used
// by SendMessage's local source modes; never called for SourceModeWeb).
type Service struct {
	sessions  domainstudy.SessionRepository
	messages  domainstudy.MessageRepository
	llm       domainllm.Provider
	profiles  domainprofile.Store
	folders   domainfolder.Repository
	retriever domainknowledge.Retriever
}

// NewService creates a Service backed by the given ports.
func NewService(
	sessions domainstudy.SessionRepository,
	messages domainstudy.MessageRepository,
	llm domainllm.Provider,
	profiles domainprofile.Store,
	folders domainfolder.Repository,
	retriever domainknowledge.Retriever,
) *Service {
	return &Service{
		sessions: sessions, messages: messages, llm: llm,
		profiles: profiles, folders: folders, retriever: retriever,
	}
}
