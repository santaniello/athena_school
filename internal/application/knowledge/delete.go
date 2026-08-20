package knowledge

import "context"

// DeleteItem permanently removes id and every chunk it owns, atomically:
// either both are gone or neither is.
//
// This never touches ingested_files: for an imported note, deleting its
// Item here has no effect on the source file, and does not un-suppress it
// on the next import (see the Domain section of
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md).
func (s *Service) DeleteItem(ctx context.Context, id string) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := s.chunks.DeleteByItemID(ctx, id); err != nil {
			return err
		}
		return s.items.Delete(ctx, id)
	})
}
