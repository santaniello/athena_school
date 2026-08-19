// Package knowledge implements use cases for extracting and saving knowledge.
package knowledge

import (
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Service implements knowledge extraction and Explorer management against
// the application's ports.
type Service struct {
	items    domainknowledge.Repository
	sessions domainstudy.SessionRepository
	messages domainstudy.MessageRepository
	llm      domainllm.Provider
	configs  domainconfig.Store
	chunks   domainknowledge.ChunkRepository
}

// NewService creates a knowledge extraction and Explorer management service.
func NewService(
	items domainknowledge.Repository,
	sessions domainstudy.SessionRepository,
	messages domainstudy.MessageRepository,
	llm domainllm.Provider,
	configs domainconfig.Store,
	chunks domainknowledge.ChunkRepository,
) *Service {
	return &Service{items: items, sessions: sessions, messages: messages, llm: llm, configs: configs, chunks: chunks}
}
