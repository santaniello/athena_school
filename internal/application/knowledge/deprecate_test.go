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

func TestDeprecate_transitionsApprovedToDeprecated_andReconcilesChunkMetadata(t *testing.T) {
	// Given an approved item, including one that arrived there as an
	// imported note (approved from creation, never having been a draft)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Status == domainknowledge.StatusDeprecated
	})).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1", Topic: "Go", Status: domainknowledge.StatusDeprecated}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated).Return(updatedChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When deprecating it
	updated, err := service.Deprecate(ctx, "item-1")

	// Then it comes back deprecated
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusDeprecated, updated.Status)
}

func TestDeprecate_returnsInvalidTransition_whenItemIsStillADraft(t *testing.T) {
	// Given a draft item (not yet approved)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Status: domainknowledge.StatusDraft,
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t))

	// When deprecating it
	_, err := service.Deprecate(ctx, "item-1")

	// Then it fails, and Update is never called
	assert.ErrorIs(t, err, domainknowledge.ErrInvalidStatusTransition)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestDeprecate_returnsErrIndexLoading_whenIndexIsLoading_andNeverTouchesTheRepository(t *testing.T) {
	// Given a loading/retrying index
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(ErrIndexLoading).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard)

	// When deprecating an item
	_, err := service.Deprecate(ctx, "item-1")

	// Then the mutation is rejected before ever reading the repository
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestDeprecate_returnsIndexingWarning_whenPostCommitReconciliationFails_butKeepsTheDurableResult(t *testing.T) {
	// Given an approved item whose transition persists successfully
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated).Return(updatedChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When deprecating it and the post-commit reconciliation fails
	updated, err := service.Deprecate(ctx, "item-1")

	// Then the durable transition is not reported as a failure
	var warning *IndexingWarning
	require.ErrorAs(t, err, &warning)
	assert.ErrorIs(t, warning.Err, boom)
	assert.Equal(t, "item-1", updated.ID)
	assert.Equal(t, domainknowledge.StatusDeprecated, updated.Status)
}
