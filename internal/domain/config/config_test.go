package config

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_WithDefaults_setsMaxKnowledgeExtractionItemsWhenUnset(t *testing.T) {
	// Given a config with no extraction limit
	cfg := Config{OpenRouterKey: "sk-or-example"}

	// When applying defaults
	got := cfg.WithDefaults()

	// Then the documented default is used without changing the key
	assert.Equal(t, "sk-or-example", got.OpenRouterKey)
	assert.Equal(t, 8, got.MaxKnowledgeExtractionItems)
}

func TestConfig_WithDefaults_preservesConfiguredExtractionLimit(t *testing.T) {
	// Given a config with an explicit extraction limit
	cfg := Config{MaxKnowledgeExtractionItems: 12}

	// When applying defaults
	got := cfg.WithDefaults()

	// Then the configured value is preserved
	assert.Equal(t, 12, got.MaxKnowledgeExtractionItems)
}

func TestConfig_Validate_acceptsExtractionLimitsWithinRange(t *testing.T) {
	for _, limit := range []int{1, 20} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			// Given a config at a valid boundary
			cfg := Config{MaxKnowledgeExtractionItems: limit}

			// When validating it
			err := cfg.Validate()

			// Then it is accepted
			require.NoError(t, err)
		})
	}
}

func TestConfig_Validate_rejectsExtractionLimitsOutsideRange(t *testing.T) {
	for _, limit := range []int{0, 21} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			// Given a config outside the valid range
			cfg := Config{MaxKnowledgeExtractionItems: limit}

			// When validating it
			err := cfg.Validate()

			// Then it is rejected with the documented error
			assert.ErrorIs(t, err, ErrMaxKnowledgeExtractionItemsOutOfRange)
		})
	}
}
