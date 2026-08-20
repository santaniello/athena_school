// Package knowledge holds the Item domain model and the Repository port
// infrastructure adapters implement. An Item is a unit of knowledge
// extracted from study sessions or imported notes — see
// specs/phases/phase-02-knowledge-engine/01-knowledge-item.md.
package knowledge

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrTopicRequired is returned when an Item has no topic.
	ErrTopicRequired = errors.New("knowledge item topic is required")
	// ErrConceptRequired is returned when an Item has no concept.
	ErrConceptRequired = errors.New("knowledge item concept is required")
	// ErrDefinitionRequired is returned when an Item has no definition.
	ErrDefinitionRequired = errors.New("knowledge item definition is required")
)

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

// NormalizeTopic trims topic's edges and rejects an empty result with
// ErrTopicRequired. Case, accents, and internal whitespace are preserved —
// this phase deliberately keeps Topic identity case-sensitive (see
// specs/phases/phase-02-knowledge-engine/04-vector-search.md); a canonical,
// case-insensitive topic key is a separate, not-yet-implemented spec. Every
// knowledge write boundary that sets Topic calls this instead of trimming
// ad hoc, so the value a Chunk's exact-match filters see is always
// consistent with the value an Item's own field holds.
func NormalizeTopic(topic string) (string, error) {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return "", ErrTopicRequired
	}
	return trimmed, nil
}

// Validate checks the fields required for a useful knowledge item.
func (i Item) Validate() error {
	if strings.TrimSpace(i.Topic) == "" {
		return ErrTopicRequired
	}
	if strings.TrimSpace(i.Concept) == "" {
		return ErrConceptRequired
	}
	if strings.TrimSpace(i.Definition) == "" {
		return ErrDefinitionRequired
	}
	return nil
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
