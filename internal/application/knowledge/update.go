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
// Transactor).
func (s *Service) UpdateItem(ctx context.Context, id string, fields ItemFields) (domainknowledge.Item, error) {
	var item domainknowledge.Item
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		item, err = s.items.GetByID(ctx, id)
		if err != nil {
			return err
		}

		item.Topic = fields.Topic
		item.Concept = fields.Concept
		item.Definition = fields.Definition
		item.Properties = fields.Properties
		item.TradeOffs = fields.TradeOffs
		item.RelatedConcepts = fields.RelatedConcepts

		if err := item.Validate(); err != nil {
			return err
		}
		item.UpdatedAt = time.Now().UTC()

		return s.items.Update(ctx, item)
	})
	if err != nil {
		return domainknowledge.Item{}, err
	}
	return item, nil
}
