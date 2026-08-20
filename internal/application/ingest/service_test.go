package ingest

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestReconcileContext_deadlineIsBoundedAroundFiveSeconds(t *testing.T) {
	// When building a reconciliation context
	before := time.Now()
	ctx, cancel := reconcileContext()
	defer cancel()

	// Then it is still open and its deadline is bounded close to 5s out
	require.NoError(t, ctx.Err())
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, before.Add(5*time.Second), deadline, 500*time.Millisecond)
}
