package knowledge

import (
	"context"
	"log"
)

// DeleteItem permanently removes id and every chunk it owns, atomically:
// either both are gone or neither is. It also cleans up any Evidence
// snapshot left with no remaining Item reference — one still linked to
// another Item survives (see EvidenceRepository.DeleteUnreferenced).
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
	if err := s.index.BeginMutation(); err != nil {
		return err
	}
	defer s.index.EndMutation()

	var removedChunkIDs []string
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		removedChunkIDs, err = s.chunks.DeleteByItemID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.items.Delete(ctx, id); err != nil {
			return err
		}
		return s.evidence.DeleteUnreferenced(ctx)
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

// DeleteSessionsWithKnowledge runs deleteSessions — which must delete every
// session in sessionIDs, or the folder holding them — while keeping the
// knowledge those sessions own consistent. The rows themselves leave through
// the sessions' ON DELETE CASCADE foreign keys; what cascade cannot reach is
// handled here:
//
//   - the chunk IDs are read first, inside the same transaction, because
//     they are gone once the sessions are — they are what the in-memory
//     VectorStore must evict;
//   - knowledge_evidence snapshots only the deleted items referenced are
//     cleaned up (see EvidenceRepository.DeleteUnreferenced) in that same
//     transaction, so a failure rolls the whole delete back;
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
		if err := deleteSessions(ctx); err != nil {
			return err
		}
		return s.evidence.DeleteUnreferenced(ctx)
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
