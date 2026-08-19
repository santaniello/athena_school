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
//
// The read and the write run inside one transaction so a concurrent
// Deprecate or UpdateItem on the same id can never read a stale copy in
// the gap and write it back over this transition (see Transactor).
func (s *Service) Approve(ctx context.Context, id string) (domainknowledge.Item, error) {
	var item domainknowledge.Item
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		item, err = s.items.GetByID(ctx, id)
		if err != nil {
			return err
		}
		item, err = item.TransitionTo(domainknowledge.StatusApproved, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.items.Update(ctx, item)
	})
	if err != nil {
		return domainknowledge.Item{}, err
	}
	return item, nil
}
