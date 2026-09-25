package folder

import (
	"context"
	"fmt"
)

// DeleteFolder deletes the folder with the given id, first deleting all of
// its sessions together with the knowledge they own — nothing in the folder
// survives. The sessions go inside the knowledge cascade so the search index
// and orphaned evidence stay consistent; the folder row is deleted after,
// outside it, because FolderRepository.Delete does not join a transaction.
func (s *Service) DeleteFolder(ctx context.Context, id string) error {
	sessions, err := s.sessions.ListByFolder(ctx, id)
	if err != nil {
		return fmt.Errorf("folder: listing sessions before delete: %w", err)
	}
	sessionIDs := make([]string, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.ID
	}
	err = s.knowledge.DeleteSessionsWithKnowledge(ctx, sessionIDs, func(ctx context.Context) error {
		return s.sessions.DeleteByFolder(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("folder: deleting sessions before delete: %w", err)
	}
	if err := s.folders.Delete(ctx, id); err != nil {
		return fmt.Errorf("folder: deleting folder: %w", err)
	}
	return nil
}
