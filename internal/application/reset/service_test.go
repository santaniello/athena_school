package reset_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/reset"
	"github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	resetmocks "github.com/santaniello/athena/internal/domain/reset/mocks"
)

func TestResetLocalData_clearsStorageThenEmptiesTheVectorIndex(t *testing.T) {
	// Given a resetter that succeeds and a vector store ready to be replaced
	resetter := resetmocks.NewMockResetter(t)
	resetter.EXPECT().Reset(context.Background()).Return(nil).Once()
	vectorStore := knowledgemocks.NewMockVectorStore(t)
	vectorStore.EXPECT().ReplaceAll(context.Background(), []knowledge.Chunk(nil)).Return(nil).Once()
	service := reset.NewService(resetter, vectorStore)

	// When resetting local data
	err := service.ResetLocalData(context.Background())

	// Then it succeeds
	require.NoError(t, err)
}

func TestResetLocalData_propagatesResetterError_withoutTouchingTheVectorIndex(t *testing.T) {
	// Given a resetter that fails
	resetter := resetmocks.NewMockResetter(t)
	resetter.EXPECT().Reset(context.Background()).Return(assert.AnError).Once()
	vectorStore := knowledgemocks.NewMockVectorStore(t)
	service := reset.NewService(resetter, vectorStore)

	// When resetting local data
	err := service.ResetLocalData(context.Background())

	// Then the error is surfaced unchanged, and the vector store is never touched
	assert.ErrorIs(t, err, assert.AnError)
}

func TestResetLocalData_propagatesVectorStoreError(t *testing.T) {
	// Given a resetter that succeeds but a vector store that fails to clear
	resetter := resetmocks.NewMockResetter(t)
	resetter.EXPECT().Reset(context.Background()).Return(nil).Once()
	vectorStore := knowledgemocks.NewMockVectorStore(t)
	vectorStore.EXPECT().ReplaceAll(context.Background(), []knowledge.Chunk(nil)).Return(assert.AnError).Once()
	service := reset.NewService(resetter, vectorStore)

	// When resetting local data
	err := service.ResetLocalData(context.Background())

	// Then the error is surfaced unchanged
	assert.ErrorIs(t, err, assert.AnError)
}
