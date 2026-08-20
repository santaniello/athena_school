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
//
// The read and the write run inside one transaction so a concurrent
// Approve or UpdateItem on the same id can never read a stale copy in the
// gap and write it back over this transition (see Transactor). The same
// transaction updates every owned chunk's status metadata; after commit,
// the VectorStore is reconciled to match without a new embedding call. A
// post-commit reconciliation failure never rolls back the durable
// transition — it comes back as an *IndexingWarning alongside the real,
// transitioned item (see IndexingWarning).
func (s *Service) Deprecate(ctx context.Context, id string) (domainknowledge.Item, error) {
	if err := s.index.CheckMutationAllowed(); err != nil {
		return domainknowledge.Item{}, err
	}

	var item domainknowledge.Item
	var updatedChunks []domainknowledge.Chunk
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		item, err = s.items.GetByID(ctx, id)
		if err != nil {
			return err
		}
		item, err = item.TransitionTo(domainknowledge.StatusDeprecated, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := s.items.Update(ctx, item); err != nil {
			return err
		}
		updatedChunks, err = s.chunks.UpdateMetadataByItemID(ctx, id, item.Topic, item.Status)
		return err
	})
	if err != nil {
		return domainknowledge.Item{}, err
	}

	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Add(reconcileCtx, updatedChunks); err != nil {
		return item, &IndexingWarning{Err: err}
	}
	return item, nil
}
