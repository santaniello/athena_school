package exercise

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxHints_isThree(t *testing.T) {
	// Given the exercise engine's hint limit
	// When reading it
	// Then an attempt may receive at most three hints
	assert.Equal(t, 3, MaxHints)
}

func TestHintErrors_areDistinctSentinels(t *testing.T) {
	// Given the two hint refusals
	// When comparing them
	// Then each is its own error with its own message
	assert.False(t, errors.Is(ErrNoHintsLeft, ErrHintInProgress))
	assert.Equal(t, "no hints left for this attempt", ErrNoHintsLeft.Error())
	assert.Equal(t, "a hint is already being requested for this attempt", ErrHintInProgress.Error())
}
