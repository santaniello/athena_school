package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestDeleteItem_cascadesToChunks_thenDeletesTheItem(t *testing.T) {
	// Given an item with chunks
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	var order []string
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Run(func(context.Context, string) {
		order = append(order, "chunks")
	}).Return(nil, nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Run(func(context.Context, string) {
		order = append(order, "item")
	}).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx)

	// When deleting it
	err := service.DeleteItem(ctx, "item-1")

	// Then both the chunks and the item are removed, chunks first
	require.NoError(t, err)
	assert.Equal(t, []string{"chunks", "item"}, order)
}

func TestDeleteItem_propagatesChunkDeletionError_withoutDeletingTheItem(t *testing.T) {
	// Given a chunk repository that fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	boom := errors.New("disk full")
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx)

	// When deleting the item
	err := service.DeleteItem(ctx, "item-1")

	// Then the error propagates and the item itself is never deleted,
	// leaving it recoverable via a retry
	assert.ErrorIs(t, err, boom)
	repository.AssertNotCalled(t, "Delete", ctx, "item-1")
}
