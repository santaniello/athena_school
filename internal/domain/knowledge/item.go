// Package knowledge holds the Item domain model and the Repository port
// infrastructure adapters implement. An Item is the record that owns an
// imported document's chunks — see
// specs/phases/phase-02-knowledge-engine/01-knowledge-item.md and
// specs/phases/phase-02-knowledge-engine/16-remove-conversation-extraction.md.
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
	// ErrSessionRequired is returned when knowledge is written without the
	// study session that owns it.
	ErrSessionRequired = errors.New("knowledge session is required")
)

// SourceImportedDoc is the only Source value: an Item comes from a document
// the user imported into a study session.
const SourceImportedDoc = "imported_doc"

// Item is a single unit of knowledge: a concept, its definition,
// and the properties/trade-offs/related concepts that describe it.
type Item struct {
	ID string
	// SessionID is the study session that owns this Item; deleting the
	// session deletes the Item.
	SessionID       string
	Topic           string
	Concept         string
	Definition      string
	Properties      []string
	TradeOffs       []string
	RelatedConcepts []string
	Source          string // imported_doc
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
