// Package reset holds the use case for permanently clearing locally-stored
// study and knowledge data. See
// specs/phases/phase-01-desktop-mvp/13-reset-local-data.md.
package reset

import (
	"context"
	"fmt"
	"log"

	"github.com/santaniello/athena/internal/domain/knowledge"
	domainreset "github.com/santaniello/athena/internal/domain/reset"
)

// Service implements ResetLocalData against a domainreset.Resetter and a
// knowledge.VectorStore.
type Service struct {
	resetter    domainreset.Resetter
	vectorStore knowledge.VectorStore
}

// NewService creates a Service backed by the given resetter and vector
// store.
func NewService(resetter domainreset.Resetter, vectorStore knowledge.VectorStore) *Service {
	return &Service{resetter: resetter, vectorStore: vectorStore}
}

// ResetLocalData permanently deletes every study session, non-default
// folder, and knowledge-domain row from durable storage, then empties the
// in-memory vector index so it stays coherent with SQLite — the same
// invariant every other knowledge mutation already keeps (see ADR-004). It
// never touches the OpenRouter config or the onboarding profile.
//
// Once the durable delete succeeds, a failure to clear the vector index is
// only logged, not returned: the caller (App.ResetLocalData) reloads the UI
// whenever this returns nil, and by that point the data is already
// irreversibly gone from SQLite. Surfacing an error here would leave the UI
// showing stale sessions/folders that no longer exist, which is worse than
// a briefly stale in-memory index that the next successful ReplaceAll (e.g.
// an import) will overwrite anyway.
func (s *Service) ResetLocalData(ctx context.Context) error {
	if err := s.resetter.Reset(ctx); err != nil {
		return fmt.Errorf("reset: clearing local storage: %w", err)
	}
	if err := s.vectorStore.ReplaceAll(ctx, nil); err != nil {
		log.Printf("reset: clearing the vector index: %v", err)
	}
	return nil
}
