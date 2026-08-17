package study

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Start opens a new study session for topic and persists it. It does not
// call the LLM: the caller requests the opening turn separately via
// RequestOpeningTurn, once the session already exists, so the UI can switch
// to the chat view immediately instead of waiting for the entire opening
// response before showing anything.
func (s *Service) Start(ctx context.Context, topic string) (domainstudy.Session, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return domainstudy.Session{}, ErrTopicRequired
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

	return session, nil
}
