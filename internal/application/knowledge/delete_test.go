package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestDeleteItem_cascadesToChunks_thenDeletesTheItem(t *testing.T) {
	// Given an item with chunks
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	service := NewService(repository, nil, nil, nil, nil, chunks)

	// When deleting it
	err := service.DeleteItem(ctx, "item-1")

	// Then both the chunks and the item are removed
	require.NoError(t, err)
}

func TestDeleteItem_propagatesChunkDeletionError_withoutDeletingTheItem(t *testing.T) {
	// Given a chunk repository that fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	boom := errors.New("disk full")
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(boom).Once()
	service := NewService(repository, nil, nil, nil, nil, chunks)

	// When deleting the item
	err := service.DeleteItem(ctx, "item-1")

	// Then the error propagates and the item itself is never deleted,
	// leaving it recoverable via a retry
	assert.ErrorIs(t, err, boom)
	repository.AssertNotCalled(t, "Delete", ctx, "item-1")
}
