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
func (s *Service) UpdateItem(ctx context.Context, id string, fields ItemFields) (domainknowledge.Item, error) {
	item, err := s.items.GetByID(ctx, id)
	if err != nil {
		return domainknowledge.Item{}, err
	}

	item.Topic = fields.Topic
	item.Concept = fields.Concept
	item.Definition = fields.Definition
	item.Properties = fields.Properties
	item.TradeOffs = fields.TradeOffs
	item.RelatedConcepts = fields.RelatedConcepts

	if err := item.Validate(); err != nil {
		return domainknowledge.Item{}, err
	}
	item.UpdatedAt = time.Now().UTC()

	if err := s.items.Update(ctx, item); err != nil {
		return domainknowledge.Item{}, err
	}
	return item, nil
}
