package folder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

// CreateFolder creates a new folder named name.
func (s *Service) CreateFolder(ctx context.Context, name string) (domainfolder.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domainfolder.Folder{}, domainfolder.ErrNameRequired
	}

	f := domainfolder.Folder{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.folders.Create(ctx, f); err != nil {
		return domainfolder.Folder{}, fmt.Errorf("folder: creating folder: %w", err)
	}
	return f, nil
}
