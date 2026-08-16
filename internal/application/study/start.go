package study

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Start opens a new study session for topic, persists it, and streams the
// assistant's opening turn (driven purely by the system prompt built from
// the current UserProfile) via onChunk.
func (s *Service) Start(ctx context.Context, topic string, onChunk func(chunk string) error) (domainstudy.Session, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return domainstudy.Session{}, ErrTopicRequired
	}

	profile, err := s.profiles.Load()
	if err != nil {
		return domainstudy.Session{}, fmt.Errorf("study: loading profile: %w", err)
	}

	session := domainstudy.Session{
		ID:        uuid.NewString(),
		Topic:     topic,
		Mode:      domainstudy.ModeStudy,
		StartedAt: time.Now().UTC(),
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return domainstudy.Session{}, fmt.Errorf("study: creating session: %w", err)
	}

	systemPrompt := buildSystemPrompt(profile, topic)
	openingTurn := []domainllm.Message{{Role: "system", Content: systemPrompt}}
	if _, err := s.streamAndPersist(ctx, session.ID, openingTurn, onChunk); err != nil {
		return session, fmt.Errorf("study: starting conversation: %w", err)
	}

	return session, nil
}
