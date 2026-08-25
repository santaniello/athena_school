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

func TestIndexingWarning_Unwrap_returnsTheUnderlyingErrorAndErrIndexingFailed(t *testing.T) {
	// Given an IndexingWarning wrapping a technical error
	boom := errors.New("store exploded")
	warning := &IndexingWarning{Err: boom}

	// When unwrapping it
	unwrapped := warning.Unwrap()

	// Then it exposes both the exact underlying error and ErrIndexingFailed,
	// so errors.Is/As can reach either
	assert.ElementsMatch(t, []error{boom, ErrIndexingFailed}, unwrapped)
}

func TestIndexingWarning_ErrorsIs_matchesErrIndexingFailed(t *testing.T) {
	// Given an IndexingWarning wrapping an arbitrary technical error
	warning := &IndexingWarning{Err: errors.New("store exploded")}
	var err error = warning

	// Then a single errors.Is(err, ErrIndexingFailed) check recognizes it,
	// the same check every knowledge-indexing failure standardizes on
	assert.ErrorIs(t, err, ErrIndexingFailed)
}
