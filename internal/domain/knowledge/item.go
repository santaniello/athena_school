// Package knowledge holds the Item domain model and the Repository port
// infrastructure adapters implement. An Item is a unit of knowledge
// extracted from study sessions or imported notes — see
// specs/phases/phase-02-knowledge-engine/01-knowledge-item.md.
package knowledge

import "time"

// Source values categorize where an Item came from. This is a category,
// not provenance — see specs/phases/phase-02-knowledge-engine/09-persistent-provenance.md
// for the evidence tables that record concrete supporting messages/chunks.
const (
	SourceAthena      = "athena"
	SourceUserNote    = "user_note"
	SourceImportedDoc = "imported_doc"
)

// Status values describe an Item's place in its lifecycle. See
// TransitionTo for the transitions allowed between them.
const (
	StatusDraft      = "draft"
	StatusApproved   = "approved"
	StatusDeprecated = "deprecated"
)

// Item is a single unit of knowledge: a concept, its definition,
// and the properties/trade-offs/related concepts that describe it.
type Item struct {
	ID              string
	Topic           string
	Concept         string
	Definition      string
	Properties      []string
	TradeOffs       []string
	RelatedConcepts []string
	Source          string // athena | user_note | imported_doc
	Status          string // draft | approved | deprecated
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TransitionTo returns a copy of i with its status moved to next, or an
// error if that transition is not allowed. now is injected so the
// function stays pure.
func (i Item) TransitionTo(next string, now time.Time) (Item, error) {
	if next != StatusDraft && next != StatusApproved && next != StatusDeprecated {
		return Item{}, ErrUnknownStatus
	}
	if !allowedTransition(i.Status, next) {
		return Item{}, ErrInvalidStatusTransition
	}
	i.Status = next
	i.UpdatedAt = now
	return i, nil
}

func allowedTransition(from, to string) bool {
	return (from == StatusDraft && to == StatusApproved) ||
		(from == StatusApproved && to == StatusDeprecated)
}
