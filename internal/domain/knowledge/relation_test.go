package knowledge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalRelation_OrdersByLexicographicallySmallerID(t *testing.T) {
	// Given two item IDs in either order
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	// When building the canonical relation between them, in both orders
	fromFirst := CanonicalRelation("item-b", "item-a", RelationRelated, now)
	fromSecond := CanonicalRelation("item-a", "item-b", RelationRelated, now)

	// Then both orders produce the exact same row — the smaller ID always
	// ends up in FromItemID — so applying the same undirected relation
	// twice never creates two different rows.
	assert.Equal(t, fromFirst, fromSecond)
	assert.Equal(t, "item-a", fromFirst.FromItemID)
	assert.Equal(t, "item-b", fromFirst.ToItemID)
}

func TestRelation_Validate_acceptsACompleteRelation(t *testing.T) {
	// Given a complete relation between two distinct items
	relation := Relation{
		FromItemID: "item-a", ToItemID: "item-b",
		Type: RelationRelated, CreatedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC),
	}

	// When validating it
	err := relation.Validate()

	// Then it is accepted
	require.NoError(t, err)
}

func TestRelation_Validate_rejectsInvalidFields(t *testing.T) {
	// Given a complete, valid relation, and a set of single-field mutations
	// away from it
	valid := Relation{
		FromItemID: "item-a", ToItemID: "item-b",
		Type: RelationRelated, CreatedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name     string
		mutate   func(*Relation)
		expected error
	}{
		{name: "from item id", mutate: func(r *Relation) { r.FromItemID = " " }, expected: ErrRelationFromItemIDRequired},
		{name: "to item id", mutate: func(r *Relation) { r.ToItemID = " " }, expected: ErrRelationToItemIDRequired},
		{name: "same item", mutate: func(r *Relation) { r.ToItemID = r.FromItemID }, expected: ErrRelationSameItem},
		{name: "type", mutate: func(r *Relation) { r.Type = " " }, expected: ErrRelationTypeRequired},
		{name: "unsupported type", mutate: func(r *Relation) { r.Type = "prerequisite" }, expected: ErrRelationTypeUnsupported},
		{name: "created at", mutate: func(r *Relation) { r.CreatedAt = time.Time{} }, expected: ErrRelationCreatedAtRequired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given the valid relation with one field mutated to be invalid
			candidate := valid
			test.mutate(&candidate)

			// When validating it
			err := candidate.Validate()

			// Then the matching domain error is returned
			assert.ErrorIs(t, err, test.expected)
		})
	}
}
