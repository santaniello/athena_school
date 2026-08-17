package study

import (
	"context"
	"fmt"
	"time"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Resume returns sessionID's full message history, reopening the session
// first if it was previously ended, so the user can keep chatting in it.
func (s *Service) Resume(ctx context.Context, sessionID string) (domainstudy.Session, []domainstudy.Message, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: finding session: %w", err)
	}

	if !session.IsOpen() {
		if err := s.sessions.Reopen(ctx, sessionID); err != nil {
			return domainstudy.Session{}, nil, fmt.Errorf("study: reopening session: %w", err)
		}
		session.EndedAt = time.Time{}
	}

	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: loading history: %w", err)
	}
	return session, history, nil
}
