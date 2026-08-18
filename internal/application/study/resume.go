package study

import (
	"context"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Resume returns sessionID's full message history, so the user can keep
// chatting in it.
func (s *Service) Resume(ctx context.Context, sessionID string) (domainstudy.Session, []domainstudy.Message, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: finding session: %w", err)
	}

	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: loading history: %w", err)
	}
	return session, history, nil
}
