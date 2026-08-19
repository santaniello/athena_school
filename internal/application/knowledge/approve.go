package knowledge

import (
	"context"
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// Approve transitions the item id from draft to approved, or returns
// domainknowledge.ErrInvalidStatusTransition if it is not currently a
// draft. Returns the updated item so callers can patch local state
// without a refetch.
func (s *Service) Approve(ctx context.Context, id string) (domainknowledge.Item, error) {
	item, err := s.items.GetByID(ctx, id)
	if err != nil {
		return domainknowledge.Item{}, err
	}
	item, err = item.TransitionTo(domainknowledge.StatusApproved, time.Now().UTC())
	if err != nil {
		return domainknowledge.Item{}, err
	}
	if err := s.items.Update(ctx, item); err != nil {
		return domainknowledge.Item{}, err
	}
	return item, nil
}
