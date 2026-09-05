package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"
)

// RelationRelated is the only relation type this phase persists — a
// symmetric "these are distinct but connected concepts" link. Phase 7 may
// add directional types (prerequisite, extends) without replacing this
// table or this constant's meaning.
const RelationRelated = "related"

// Relation persistence invariant errors, returned by Relation.Validate.
var (
	ErrRelationFromItemIDRequired = errors.New("relation from item id is required")
	ErrRelationToItemIDRequired   = errors.New("relation to item id is required")
	ErrRelationSameItem           = errors.New("relation from and to item must be different items")
	ErrRelationTypeRequired       = errors.New("relation type is required")
	ErrRelationTypeUnsupported    = errors.New("relation type is not supported in this phase")
	ErrRelationCreatedAtRequired  = errors.New("relation created at is required")
)

// Relation links two distinct Knowledge Items. See
// specs/phases/phase-02-knowledge-engine/11-knowledge-reconciliation.md.
type Relation struct {
	FromItemID string
	ToItemID   string
	Type       string
	CreatedAt  time.Time
}

// CanonicalRelation returns the Relation between a and b, ordered so
// FromItemID is always the lexicographically smaller of the two IDs.
// RelationRelated is symmetric — applying it to (a, b) and to (b, a) must
// persist as the exact same row, never two, so callers always build it
// through this constructor rather than the Relation literal directly.
func CanonicalRelation(a, b, relationType string, now time.Time) Relation {
	if b < a {
		a, b = b, a
	}
	return Relation{FromItemID: a, ToItemID: b, Type: relationType, CreatedAt: now}
}

// Validate checks the persistence invariants of a Relation: both item ids
// present and distinct, a relation type, and a CreatedAt timestamp.
func (r Relation) Validate() error {
	if strings.TrimSpace(r.FromItemID) == "" {
		return ErrRelationFromItemIDRequired
	}
	if strings.TrimSpace(r.ToItemID) == "" {
		return ErrRelationToItemIDRequired
	}
	if r.FromItemID == r.ToItemID {
		return ErrRelationSameItem
	}
	if strings.TrimSpace(r.Type) == "" {
		return ErrRelationTypeRequired
	}
	if r.Type != RelationRelated {
		return ErrRelationTypeUnsupported
	}
	if r.CreatedAt.IsZero() {
		return ErrRelationCreatedAtRequired
	}
	return nil
}

// RelationRepository persists Relation rows between Items.
type RelationRepository interface {
	// Save persists relation. It is idempotent: saving the same canonical
	// relation twice — same FromItemID, ToItemID and Type — must succeed
	// without erroring or duplicating the row, mirroring the table's
	// composite primary key.
	Save(ctx context.Context, relation Relation) error
}
