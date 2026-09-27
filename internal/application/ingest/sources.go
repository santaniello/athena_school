package ingest

import (
	"context"
	"errors"
	"fmt"
	"log"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// ErrSourceNotFound is returned by RemoveSource when itemID does not exist
// at all, or exists but belongs to a different session. The two cases are
// deliberately indistinguishable to the caller, so one session can never
// probe another's document ids.
var ErrSourceNotFound = errors.New("ingest: source not found")

// ListSources returns sessionID's imported documents for the Sources
// panel, oldest-imported first. See
// specs/phases/phase-02-knowledge-engine/17-session-sources-panel.md.
func (s *Service) ListSources(ctx context.Context, sessionID string) ([]domainknowledge.SessionSource, error) {
	sources, err := s.ingestedFiles.ListSourcesBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("ingest: listing session sources: %w", err)
	}
	return sources, nil
}

// RemoveSource hard-deletes itemID's chunks, its shadow knowledge.Item and
// its ingested_files record — all scoped to sessionID — evicting the
// removed chunks from the in-memory index once the transaction commits. It
// never touches the source file on disk, and never touches another
// session's copy of the same file.
//
// Deleting the ingested_files record (unlike the old, deleted DeleteItem —
// spec 2.3, which deliberately kept it) means importing the same,
// unchanged file again re-ingests it instead of being skipped as
// unchanged: an explicit Remove forgets the import, not just the
// knowledge derived from it.
//
// itemID not existing, or belonging to another session, is reported the
// same way — ErrSourceNotFound — before any mutation.
//
// A post-commit eviction failure never undoes the durable delete and is
// only logged, same rule as DeleteSessionsWithKnowledge: the row is
// already gone, so a leftover in-memory chunk is unreachable once
// retrieval re-checks the item.
func (s *Service) RemoveSource(ctx context.Context, sessionID, itemID string) error {
	if err := s.index.BeginMutation(); err != nil {
		return err
	}
	defer s.index.EndMutation()

	item, err := s.items.GetByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, domainknowledge.ErrItemNotFound) {
			return ErrSourceNotFound
		}
		return fmt.Errorf("ingest: looking up source: %w", err)
	}
	if item.SessionID != sessionID {
		return ErrSourceNotFound
	}

	var chunkIDs []string
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		chunkIDs, err = s.chunks.DeleteByItemID(ctx, itemID)
		if err != nil {
			return err
		}
		if err := s.items.Delete(ctx, itemID); err != nil {
			return err
		}
		return s.ingestedFiles.DeleteByItemID(ctx, sessionID, itemID)
	})
	if err != nil {
		return fmt.Errorf("ingest: removing source: %w", err)
	}

	if len(chunkIDs) == 0 {
		return nil
	}
	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Remove(reconcileCtx, chunkIDs); err != nil {
		log.Printf("knowledge index: evicting chunks of removed source %s: %v", itemID, err)
	}
	return nil
}
