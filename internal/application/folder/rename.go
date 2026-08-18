package folder

import (
	"context"
	"fmt"
	"strings"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

// RenameFolder renames the folder with the given id, including the default
// folder — only deleting it is blocked, not renaming it.
func (s *Service) RenameFolder(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return domainfolder.ErrNameRequired
	}
	if err := s.folders.Rename(ctx, id, name); err != nil {
		return fmt.Errorf("folder: renaming folder: %w", err)
	}
	return nil
}
