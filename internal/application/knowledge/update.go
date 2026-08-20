package knowledge

import (
	"context"
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
// Transactor). The same transaction updates every owned chunk's topic
// metadata (editing an imported note's concept/definition never touches
// its raw chunk embeddings — only topic/status metadata mirrors the Item);
// after commit, the VectorStore is reconciled to match without a new
// embedding call. A post-commit reconciliation failure never rolls back
// the durable update — it comes back as an *IndexingWarning alongside the
// real, updated item (see IndexingWarning).
func (s *Service) UpdateItem(ctx context.Context, id string, fields ItemFields) (domainknowledge.Item, error) {
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

		topic, err := domainknowledge.NormalizeTopic(fields.Topic)
		if err != nil {
			return err
		}

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
