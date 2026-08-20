package knowledge

import "context"

// DeleteItem permanently removes id and every chunk it owns, atomically:
// either both are gone or neither is.
//
// This never touches ingested_files: for an imported note, deleting its
// Item here has no effect on the source file, and does not un-suppress it
// on the next import (see the Domain section of
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md).
//
// After the transaction commits, the removed chunk IDs are evicted from
// the VectorStore — never before commit, so a rolled-back transaction
// leaves the previous in-memory snapshot untouched. A post-commit
// reconciliation failure never undoes the durable delete — it comes back
// as an *IndexingWarning (see IndexingWarning).
func (s *Service) DeleteItem(ctx context.Context, id string) error {
	if err := s.index.CheckMutationAllowed(); err != nil {
		return err
	}

	var removedChunkIDs []string
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		removedChunkIDs, err = s.chunks.DeleteByItemID(ctx, id)
		if err != nil {
			return err
		}
		return s.items.Delete(ctx, id)
	})
	if err != nil {
		return err
	}

	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Remove(reconcileCtx, removedChunkIDs); err != nil {
		return &IndexingWarning{Err: err}
	}
	return nil
}
