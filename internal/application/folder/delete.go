package folder

import (
	"context"
	"fmt"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

// DeleteFolder deletes the folder with the given id, first reassigning all
// of its sessions to the default folder — sessions are never deleted. The
// default folder itself cannot be deleted, since it must always exist as
// the fallback target.
func (s *Service) DeleteFolder(ctx context.Context, id string) error {
	if id == domainfolder.DefaultFolderID {
		return domainfolder.ErrCannotDeleteDefaultFolder
	}
	if err := s.sessions.ReassignFolder(ctx, id, domainfolder.DefaultFolderID); err != nil {
		return fmt.Errorf("folder: reassigning sessions before delete: %w", err)
	}
	if err := s.folders.Delete(ctx, id); err != nil {
		return fmt.Errorf("folder: deleting folder: %w", err)
	}
	return nil
}
