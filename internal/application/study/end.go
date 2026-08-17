package study

import (
	"context"
	"fmt"
	"time"
)

// End closes the session gracefully by setting its EndedAt timestamp.
func (s *Service) End(ctx context.Context, sessionID string) error {
	if err := s.sessions.End(ctx, sessionID, time.Now().UTC()); err != nil {
		return fmt.Errorf("study: ending session: %w", err)
	}
	return nil
}
