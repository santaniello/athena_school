package folder

import (
	"context"
	"fmt"
)

// DeleteFolder deletes the folder with the given id, first deleting all of
// its sessions — nothing in the folder survives.
func (s *Service) DeleteFolder(ctx context.Context, id string) error {
	if err := s.sessions.DeleteByFolder(ctx, id); err != nil {
		return fmt.Errorf("folder: deleting sessions before delete: %w", err)
	}
	if err := s.folders.Delete(ctx, id); err != nil {
		return fmt.Errorf("folder: deleting folder: %w", err)
	}
	return nil
}
