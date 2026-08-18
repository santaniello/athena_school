package study

import (
	"context"
	"fmt"
)

// DeleteSession permanently deletes sessionID and every message in it.
// Unlike End, this cannot be undone.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	if err := s.messages.DeleteBySession(ctx, sessionID); err != nil {
		return fmt.Errorf("study: deleting session messages: %w", err)
	}
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("study: deleting session: %w", err)
	}
	return nil
}
