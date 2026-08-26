package study

import (
	"context"
	"fmt"
)

// DeleteSession permanently deletes sessionID. SessionRepository owns its
// dependent cleanup atomically, so the use case never exposes a partially
// deleted session.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("study: deleting session: %w", err)
	}
	return nil
}
