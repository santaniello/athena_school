package knowledge

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexingWarning_Error_includesTheUnderlyingErrorMessage(t *testing.T) {
	// Given an IndexingWarning wrapping a technical error
	warning := &IndexingWarning{Err: errors.New("store exploded")}

	// When formatting it
	message := warning.Error()

	// Then the underlying technical detail is included
	assert.Contains(t, message, "store exploded")
}

func TestIndexingWarning_Unwrap_returnsTheUnderlyingError(t *testing.T) {
	// Given an IndexingWarning wrapping a technical error
	boom := errors.New("store exploded")
	warning := &IndexingWarning{Err: boom}

	// When unwrapping it
	unwrapped := warning.Unwrap()

	// Then it returns the exact underlying error, so errors.Is/As can reach it
	assert.Same(t, boom, unwrapped)
}
