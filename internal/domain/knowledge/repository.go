package knowledge

import (
	"context"
	"errors"
)

// ErrItemNotFound is returned when no knowledge item matches the given ID.
var ErrItemNotFound = errors.New("knowledge item not found")

// Repository persists Items. Today the only implementation is
// SQLite-backed (internal/infrastructure/sqlite).
type Repository interface {
	Save(ctx context.Context, item Item) error
	// GetByID returns the item with the given id, or ErrItemNotFound if
	// it does not exist.
	GetByID(ctx context.Context, id string) (Item, error)
	// Update persists every field of item, or returns ErrItemNotFound if
	// it does not exist.
	Update(ctx context.Context, item Item) error
	// Delete permanently removes the item with the given id, or returns
	// ErrItemNotFound if it does not exist.
	Delete(ctx context.Context, id string) error
}
