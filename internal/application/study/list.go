package study

import (
	"context"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// ListSessionsByFolder returns every session in the given folder.
func (s *Service) ListSessionsByFolder(ctx context.Context, folderID string) ([]domainstudy.Session, error) {
	sessions, err := s.sessions.ListByFolder(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("study: listing sessions by folder: %w", err)
	}
	return sessions, nil
}
