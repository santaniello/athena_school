package knowledge

import (
	"context"

	"github.com/stretchr/testify/mock"

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
