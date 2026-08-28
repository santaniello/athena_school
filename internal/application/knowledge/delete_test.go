package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestDeleteItem_cascadesToChunks_thenDeletesTheItem_thenCleansUpUnreferencedEvidence_thenReconcilesTheStore(t *testing.T) {
	// Given an item with chunks
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	var order []string
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Run(func(context.Context, string) {
		order = append(order, "chunks")
	}).Return([]string{"chunk-1", "chunk-2"}, nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Run(func(context.Context, string) {
		order = append(order, "item")
	}).Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().DeleteUnreferenced(ctx).Run(func(context.Context) {
		order = append(order, "evidence")
	}).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1", "chunk-2"}).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deleting it
	err := service.DeleteItem(ctx, "item-1")

	// Then chunks and the item are removed inside the transaction, chunks
	// first, unreferenced evidence is cleaned up last, and only after
	// commit is the store reconciled
	require.NoError(t, err)
	assert.Equal(t, []string{"chunks", "item", "evidence"}, order)
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
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deleting the item
	err := service.DeleteItem(ctx, "item-1")

	// Then the error propagates and the item itself is never deleted,
	// leaving it recoverable via a retry
	assert.ErrorIs(t, err, boom)
	repository.AssertNotCalled(t, "Delete", ctx, "item-1")
}

func TestDeleteItem_propagatesEvidenceCleanupError_insideTheSameTransaction(t *testing.T) {
	// Given chunks and the item delete successfully, but evidence cleanup fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	boom := errors.New("disk full")
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().DeleteUnreferenced(ctx).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deleting the item
	err := service.DeleteItem(ctx, "item-1")

	// Then the error propagates — the enclosing transaction rolls back both
	// the chunk and item deletes along with the failed evidence cleanup
	assert.ErrorIs(t, err, boom)
}

func TestDeleteItem_returnsErrIndexLoading_whenIndexIsLoading_andNeverTouchesTheRepository(t *testing.T) {
	// Given a loading/retrying index
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(ErrIndexLoading).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deleting an item
	err := service.DeleteItem(ctx, "item-1")

	// Then the mutation is rejected before ever touching the repository
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteItem_returnsIndexingWarning_whenPostCommitReconciliationFails_butKeepsTheDurableResult(t *testing.T) {
	// Given an item whose durable delete succeeds
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	repository.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().DeleteUnreferenced(ctx).Return(nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deleting it and the post-commit reconciliation fails
	err := service.DeleteItem(ctx, "item-1")

	// Then the durable delete is not reported as a failure — it comes back
	// as a typed indexing warning instead
	var warning *IndexingWarning
	require.ErrorAs(t, err, &warning)
	assert.ErrorIs(t, warning.Err, boom)
}
