package study

import (
	"context"
	"fmt"
)

// DeleteSession permanently deletes sessionID together with the knowledge it
// owns. SessionRepository owns its dependent cleanup atomically, so the use
// case never exposes a partially deleted session; the knowledge cascade
// keeps the search index and orphaned evidence consistent around it.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	err := s.knowledge.DeleteSessionsWithKnowledge(ctx, []string{sessionID}, func(ctx context.Context) error {
		return s.sessions.Delete(ctx, sessionID)
	})
	if err != nil {
		return fmt.Errorf("study: deleting session: %w", err)
	}
	return nil
}
