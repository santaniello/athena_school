package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeConcept_trimsLowercasesAndCollapsesSeparators(t *testing.T) {
	// Given a concept with surrounding whitespace, mixed case, and punctuation
	// When normalizing it
	normalized := NormalizeConcept(" Cache-Aside  Pattern ")

	// Then it becomes the single-spaced, lowercase, punctuation-free form
	assert.Equal(t, "cache aside pattern", normalized)
}

func TestNormalizeConcept_preservesAccents(t *testing.T) {
	// Given a concept containing accented letters
	// When normalizing it
	normalized := NormalizeConcept("Padrão de Repositório")

	// Then accents are preserved — only case and separators are normalized
	assert.Equal(t, "padrão de repositório", normalized)
}

func TestNormalizeConcept_collapsesRepeatedSeparatorsIntoOneSpace(t *testing.T) {
	// Given a concept with several consecutive separators
	// When normalizing it
	normalized := NormalizeConcept("Circuit___Breaker!!!Pattern")

	// Then each run collapses into a single space
	assert.Equal(t, "circuit breaker pattern", normalized)
}

func TestNormalizeConcept_returnsEmptyForBlankInput(t *testing.T) {
	// Given a blank concept
	// When normalizing it
	normalized := NormalizeConcept("   ")

	// Then it normalizes to an empty string
	assert.Equal(t, "", normalized)
}
