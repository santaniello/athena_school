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
// transition — it comes back as an error wrapping ErrIndexingFailed
// alongside the real, transitioned item.
//
// If id owns no chunk yet (UpdateMetadataByItemID matched zero rows), the
// transition still commits and indexKnowledgeItem attempts the recoverable
// embedding afterwards, instead of reconciling an empty metadata update.
func (s *Service) Deprecate(ctx context.Context, id string) (domainknowledge.Item, error) {
	if err := s.index.BeginMutation(); err != nil {
		return domainknowledge.Item{}, err
	}
	defer s.index.EndMutation()

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
		updatedChunks, err = s.chunks.UpdateMetadataByItemID(ctx, id, item.Topic, item.Status, item.UpdatedAt)
		return err
	})
	if err != nil {
		return domainknowledge.Item{}, err
	}

	if len(updatedChunks) == 0 {
		if err := s.indexKnowledgeItem(ctx, item); err != nil {
			return item, err
		}
		return item, nil
	}

	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Add(reconcileCtx, updatedChunks); err != nil {
		return item, &IndexingWarning{Err: err}
	}
	return item, nil
}
