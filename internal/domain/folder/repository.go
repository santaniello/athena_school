package folder

import (
	"context"
	"errors"
)

// ErrFolderNotFound is returned when no folder matches the given ID.
var ErrFolderNotFound = errors.New("folder not found")

// ErrNameRequired is returned when a folder name is blank.
var ErrNameRequired = errors.New("folder name is required")

// ErrCannotDeleteDefaultFolder is returned when trying to delete the
// default folder, which must always exist as the fallback for sessions.
var ErrCannotDeleteDefaultFolder = errors.New("the default folder cannot be deleted")

// Repository persists Folders. Today the only implementation is
// SQLite-backed (internal/infrastructure/sqlite).
type Repository interface {
	Create(ctx context.Context, f Folder) error
	Rename(ctx context.Context, id, name string) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]Folder, error)
	GetByID(ctx context.Context, id string) (Folder, error)
}
