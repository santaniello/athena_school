package knowledge

import (
	"context"
	"slices"
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// ItemFields are the user-editable fields of a knowledge Item — never
// Status, Source or CreatedAt, which are server-owned and lifecycle-managed.
type ItemFields struct {
	Topic           string
	Concept         string
	Definition      string
	Properties      []string
	TradeOffs       []string
	RelatedConcepts []string
}

// UpdateItem overwrites id's editable fields, validates the result, and
// restamps UpdatedAt. Status, Source and CreatedAt are preserved from the
// existing record untouched.
//
// The read and the write run inside one transaction so a concurrent
// Approve or Deprecate on the same id can never have its transition
// overwritten by this call reading a stale Status in the gap (see
// Transactor).
//
// Re-embedding only happens when Concept, Definition, Properties or
// TradeOffs actually changed — those are the only fields the chunk's
// content is rendered from (see indexKnowledgeItem/renderItemContent); a
// Topic-only (or RelatedConcepts-only) edit stays on the cheaper
// metadata-only path Approve/Deprecate already use, updating the chunk's
// topic and ItemUpdatedAt without a new embedding call. Either way
// UpdatedAt — and so the chunk's ItemUpdatedAt — is restamped on every
// call, content-changing or not, so a Topic-only edit never makes the
// chunk look stale at the next startup.
//
// When content changed: the old chunk is deleted in the same transaction
// as the item write, evicted from the VectorStore immediately once that
// commits — before the embedding call — so retrieval and duplicate
// detection never serve stale content even while re-indexing is in
// flight, then indexKnowledgeItem re-embeds and re-inserts it. A failed
// re-embed leaves the item persisted with zero chunks (visible to
// backfill) rather than a stale one. Either path's post-commit failure
// comes back as an error wrapping ErrIndexingFailed alongside the real,
// updated item.
func (s *Service) UpdateItem(ctx context.Context, id string, fields ItemFields) (domainknowledge.Item, error) {
	if err := s.index.BeginMutation(); err != nil {
		return domainknowledge.Item{}, err
	}
	defer s.index.EndMutation()

	var item domainknowledge.Item
	var updatedChunks []domainknowledge.Chunk
	var removedChunkIDs []string
	var contentChanged bool
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		item, err = s.items.GetByID(ctx, id)
		if err != nil {
			return err
		}

		topic, err := domainknowledge.NormalizeTopic(fields.Topic)
		if err != nil {
			return err
		}

		contentChanged = fields.Concept != item.Concept ||
			fields.Definition != item.Definition ||
			!slices.Equal(fields.Properties, item.Properties) ||
			!slices.Equal(fields.TradeOffs, item.TradeOffs)

		item.Topic = topic
		item.Concept = fields.Concept
		item.Definition = fields.Definition
		item.Properties = fields.Properties
		item.TradeOffs = fields.TradeOffs
		item.RelatedConcepts = fields.RelatedConcepts

		if err := item.Validate(); err != nil {
			return err
		}
		item.UpdatedAt = time.Now().UTC()

		if err := s.items.Update(ctx, item); err != nil {
			return err
		}

		if contentChanged {
			removedChunkIDs, err = s.chunks.DeleteByItemID(ctx, id)
			return err
		}
		updatedChunks, err = s.chunks.UpdateMetadataByItemID(ctx, id, item.Topic, item.Status, item.UpdatedAt)
		return err
	})
	if err != nil {
		return domainknowledge.Item{}, err
	}

	if contentChanged {
		reconcileCtx, cancel := reconcileContext()
		evictErr := s.store.Remove(reconcileCtx, removedChunkIDs)
		cancel()
		if evictErr != nil {
			return item, indexingFailure(id, "evicting stale chunk for", evictErr)
		}
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
