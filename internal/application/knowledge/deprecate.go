package knowledge

import (
	"context"
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// Deprecate transitions the item id from approved to deprecated, or
// returns domainknowledge.ErrInvalidStatusTransition if it is not
// currently approved. Returns the updated item so callers can patch local
// state without a refetch.
func (s *Service) Deprecate(ctx context.Context, id string) (domainknowledge.Item, error) {
	item, err := s.items.GetByID(ctx, id)
	if err != nil {
		return domainknowledge.Item{}, err
	}
	item, err = item.TransitionTo(domainknowledge.StatusDeprecated, time.Now().UTC())
	if err != nil {
		return domainknowledge.Item{}, err
	}
	if err := s.items.Update(ctx, item); err != nil {
		return domainknowledge.Item{}, err
	}
	return item, nil
}
