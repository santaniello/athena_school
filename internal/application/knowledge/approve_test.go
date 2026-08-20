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

func TestApprove_transitionsDraftToApproved_andReconcilesChunkMetadata(t *testing.T) {
	// Given a draft item owning one chunk
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusDraft,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Status == domainknowledge.StatusApproved && !item.UpdatedAt.IsZero()
	})).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1", Topic: "Go", Status: domainknowledge.StatusApproved}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusApproved).Return(updatedChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	// The reconciliation context is an internal post-commit context, not
	// the caller's ctx — there is no viable exact matcher for it.
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When approving it
	updated, err := service.Approve(ctx, "item-1")

	// Then it comes back approved and the chunk metadata was reconciled
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusApproved, updated.Status)
}

func TestApprove_returnsInvalidTransition_whenItemIsAlreadyApproved(t *testing.T) {
	// Given an already-approved item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Status: domainknowledge.StatusApproved,
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t))

	// When approving it again
	_, err := service.Approve(ctx, "item-1")

	// Then it fails, and Update is never called
	assert.ErrorIs(t, err, domainknowledge.ErrInvalidStatusTransition)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestApprove_propagatesNotFound_whenItemDoesNotExist(t *testing.T) {
	// Given no matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t))

	// When approving it
	_, err := service.Approve(ctx, "missing")

	// Then the not-found error propagates
	assert.ErrorIs(t, err, domainknowledge.ErrItemNotFound)
}

func TestApprove_returnsErrIndexLoading_whenIndexIsLoading_andNeverTouchesTheRepository(t *testing.T) {
	// Given a loading/retrying index
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(ErrIndexLoading).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard)

	// When approving an item
	_, err := service.Approve(ctx, "item-1")

	// Then the mutation is rejected before ever reading the repository —
	// a retry snapshot can never be overwritten by a concurrent change
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestApprove_returnsIndexingWarning_whenPostCommitReconciliationFails_butKeepsTheDurableResult(t *testing.T) {
	// Given a draft item whose transition persists successfully
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusDraft,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusApproved).Return(updatedChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When approving it and the post-commit reconciliation fails
	updated, err := service.Approve(ctx, "item-1")

	// Then the durable transition is not reported as a failure — it comes
	// back as a typed indexing warning alongside the real, transitioned item
	var warning *IndexingWarning
	require.ErrorAs(t, err, &warning)
	assert.ErrorIs(t, warning.Err, boom)
	assert.Equal(t, "item-1", updated.ID)
	assert.Equal(t, domainknowledge.StatusApproved, updated.Status)
}
