package folder

import (
	"context"
	"fmt"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

// ListFolders returns every folder.
func (s *Service) ListFolders(ctx context.Context) ([]domainfolder.Folder, error) {
	folders, err := s.folders.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("folder: listing folders: %w", err)
	}
	return folders, nil
}
