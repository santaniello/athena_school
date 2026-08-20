package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
)

// runWithinTx makes the mocked Transactor behave like the real one: it
// just invokes fn immediately against ctx, so the repo mocks set up
// underneath faithfully observe every call Approve/Deprecate/UpdateItem/
// DeleteItem make inside their transactional read-modify-write.
func runWithinTx(tx *txmocks.MockTransactor) {
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

// passingIndexGuard returns an IndexGuard mock that always allows the
// mutation — the default for every test that isn't specifically about the
// guard rejecting one.
func passingIndexGuard(t *testing.T) *txmocks.MockIndexGuard {
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(nil)
	return guard
}

func TestReconcileContext_deadlineIsBoundedAroundFiveSeconds(t *testing.T) {
	// When building a reconciliation context
	before := time.Now()
	ctx, cancel := reconcileContext()
	defer cancel()

	// Then it is still open and its deadline is bounded close to 5s out —
	// a mutated timeout (e.g. 0 or a wildly different duration) would fail
	// either the immediate-cancellation check or the bound below
	require.NoError(t, ctx.Err())
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, before.Add(5*time.Second), deadline, 500*time.Millisecond)
}
