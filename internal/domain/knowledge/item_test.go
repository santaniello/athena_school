package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItem_Validate_acceptsRequiredFields(t *testing.T) {
	// Given an item with every required field
	item := Item{Topic: "Distributed systems", Concept: "CAP theorem", Definition: "A consistency trade-off."}

	// When validating it
	err := item.Validate()

	// Then it is accepted
	require.NoError(t, err)
}

func TestItem_Validate_returnsTopicRequiredWhenBlank(t *testing.T) {
	// Given an item with a blank topic
	item := Item{Topic: "  ", Concept: "CAP theorem", Definition: "A consistency trade-off."}

	// When validating it
	err := item.Validate()

	// Then the topic error is returned
	assert.ErrorIs(t, err, ErrTopicRequired)
}

func TestItem_Validate_returnsConceptRequiredWhenBlank(t *testing.T) {
	// Given an item with a blank concept
	item := Item{Topic: "Distributed systems", Concept: "  ", Definition: "A consistency trade-off."}

	// When validating it
	err := item.Validate()

	// Then the concept error is returned
	assert.ErrorIs(t, err, ErrConceptRequired)
}

func TestItem_Validate_returnsDefinitionRequiredWhenBlank(t *testing.T) {
	// Given an item with a blank definition
	item := Item{Topic: "Distributed systems", Concept: "CAP theorem", Definition: "  "}

	// When validating it
	err := item.Validate()

	// Then the definition error is returned
	assert.ErrorIs(t, err, ErrDefinitionRequired)
}

func TestNormalizeTopic_trimsLeadingAndTrailingWhitespace(t *testing.T) {
	// Given a topic with surrounding whitespace
	// When normalizing it
	topic, err := NormalizeTopic("  Distributed systems  ")

	// Then it is trimmed, and case/internal whitespace/accents are preserved
	require.NoError(t, err)
	assert.Equal(t, "Distributed systems", topic)
}

func TestNormalizeTopic_preservesCase(t *testing.T) {
	// Given topics differing only by case
	// When normalizing each
	lower, lowerErr := NormalizeTopic("go")
	upper, upperErr := NormalizeTopic("Go")

	// Then neither is case-folded — they remain distinct topics in this phase
	require.NoError(t, lowerErr)
	require.NoError(t, upperErr)
	assert.Equal(t, "go", lower)
	assert.Equal(t, "Go", upper)
}

func TestNormalizeTopic_returnsErrTopicRequired_whenEmpty(t *testing.T) {
	// Given an empty topic
	// When normalizing it
	_, err := NormalizeTopic("")

	// Then it is rejected
	assert.ErrorIs(t, err, ErrTopicRequired)
}

func TestNormalizeTopic_returnsErrTopicRequired_whenWhitespaceOnly(t *testing.T) {
	// Given a whitespace-only topic
	// When normalizing it
	_, err := NormalizeTopic("   \t  ")

	// Then it is rejected rather than trimmed down to an accepted empty string
	assert.ErrorIs(t, err, ErrTopicRequired)
}
