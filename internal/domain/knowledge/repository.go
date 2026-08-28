package knowledge

import (
	"context"
	"errors"
)

// ErrItemNotFound is returned when no knowledge item matches the given ID.
var ErrItemNotFound = errors.New("knowledge item not found")

// ErrInvalidStatusTransition is returned by TransitionTo when the
// requested status change is not one of the allowed transitions.
var ErrInvalidStatusTransition = errors.New("invalid knowledge item status transition")

// ErrUnknownStatus is returned by TransitionTo when the requested status
// is not one of the three known statuses.
var ErrUnknownStatus = errors.New("unknown knowledge item status")

// Filter narrows a List query. An empty field means no constraint on it.
type Filter struct{ Topic, Status string }

// Repository persists Items. Today the only implementation is
// SQLite-backed (internal/infrastructure/sqlite).
type Repository interface {
	Save(ctx context.Context, item Item) error
	// GetByID returns the item with the given id, or ErrItemNotFound if
	// it does not exist.
	GetByID(ctx context.Context, id string) (Item, error)
	// FindByTopic returns every item for the given topic, oldest first.
	FindByTopic(ctx context.Context, topic string) ([]Item, error)
	// FindByNormalizedConcept returns every item in topic whose persisted
	// normalized_concept column equals normalizedConcept — draft, approved,
	// and deprecated items alike, so exact-match duplicate detection never
	// needs an embedding call. normalizedConcept must already be the output
	// of NormalizeConcept; this method does no normalization of its own.
	FindByNormalizedConcept(ctx context.Context, topic, normalizedConcept string) ([]Item, error)
	// List returns every item matching filter, oldest first.
	List(ctx context.Context, filter Filter) ([]Item, error)
	// ListTopics returns every distinct topic, alphabetically.
	ListTopics(ctx context.Context) ([]string, error)
	// CountByStatus returns how many items currently have the given status.
	CountByStatus(ctx context.Context, status string) (int, error)
	// Update persists every field of item, or returns ErrItemNotFound if
	// it does not exist.
	Update(ctx context.Context, item Item) error
	// Delete permanently removes the item with the given id, or returns
	// ErrItemNotFound if it does not exist.
	Delete(ctx context.Context, id string) error
	// CountUnindexed returns how many Source == athena items have no
	// current chunk under embeddingModel — missing entirely, or stale by
	// ItemUpdatedAt, Status, or embedding model. Imported-doc shadow Items
	// are excluded: their freshness is governed by ingested_files (2.3),
	// never by ItemUpdatedAt, which is always NULL for them by design.
	CountUnindexed(ctx context.Context, embeddingModel string) (int, error)
	// ListUnindexed returns every item CountUnindexed would count, oldest
	// first — the backlog ReindexKnowledgeItems processes.
	ListUnindexed(ctx context.Context, embeddingModel string) ([]Item, error)
}
