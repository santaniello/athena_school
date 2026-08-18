package study

import (
	"context"
	"fmt"
)

// MoveToFolder reassigns sessionID to folderID.
func (s *Service) MoveToFolder(ctx context.Context, sessionID, folderID string) error {
	if err := s.sessions.MoveToFolder(ctx, sessionID, folderID); err != nil {
		return fmt.Errorf("study: moving session to folder: %w", err)
	}
	return nil
}
