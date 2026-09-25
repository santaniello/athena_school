package knowledge

import (
	"context"
	"log"
)

// DeleteSessionsWithKnowledge runs deleteSessions — which must delete every
// session in sessionIDs, or the folder holding them — while keeping the
// knowledge those sessions own consistent. The rows themselves leave through
// the sessions' ON DELETE CASCADE foreign keys; what cascade cannot reach is
// handled here:
//
//   - the chunk IDs are read first, inside the same transaction, because
//     they are gone once the sessions are — they are what the in-memory
//     VectorStore must evict;
//   - the VectorStore is evicted only after the transaction commits, never
//     before, so a rolled-back delete leaves the previous snapshot untouched.
//
// A post-commit eviction failure never undoes the durable delete and is only
// logged: the sessions no longer exist, and retrieval is scoped to a session,
// so a leftover in-memory chunk is unreachable and is gone at the next load.
func (s *Service) DeleteSessionsWithKnowledge(
	ctx context.Context, sessionIDs []string, deleteSessions func(ctx context.Context) error,
) error {
	if err := s.index.BeginMutation(); err != nil {
		return err
	}
	defer s.index.EndMutation()

	var chunkIDs []string
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		for _, sessionID := range sessionIDs {
			ids, err := s.chunks.ListIDsBySession(ctx, sessionID)
			if err != nil {
				return err
			}
			chunkIDs = append(chunkIDs, ids...)
		}
		return deleteSessions(ctx)
	})
	if err != nil {
		return err
	}

	if len(chunkIDs) == 0 {
		return nil
	}
	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Remove(reconcileCtx, chunkIDs); err != nil {
		log.Printf("knowledge index: evicting chunks of deleted sessions: %v", err)
	}
	return nil
}
