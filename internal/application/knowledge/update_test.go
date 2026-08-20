package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestUpdateItem_overwritesEditableFields_andReconcilesChunkMetadata(t *testing.T) {
	// Given an existing approved item owning one chunk
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	originalCreatedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
		CreatedAt: originalCreatedAt, UpdatedAt: originalCreatedAt,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Concept == "New" && item.Definition == "New def." &&
			item.Status == domainknowledge.StatusApproved && item.Source == domainknowledge.SourceAthena &&
			item.CreatedAt.Equal(originalCreatedAt) && item.UpdatedAt.After(originalCreatedAt)
	})).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1", Topic: "Go", Status: domainknowledge.StatusApproved}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusApproved).Return(updatedChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When updating its editable fields
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "Go", Concept: "New", Definition: "New def.",
		Properties: []string{"p1"}, TradeOffs: []string{"t1"}, RelatedConcepts: []string{"r1"},
	})

	// Then the change persists and Status/Source/CreatedAt are untouched
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Concept)
	assert.Equal(t, domainknowledge.StatusApproved, updated.Status)
	assert.Equal(t, domainknowledge.SourceAthena, updated.Source)
	assert.True(t, updated.CreatedAt.Equal(originalCreatedAt))
}

func TestUpdateItem_returnsValidationError_whenConceptIsCleared_andNeverCallsUpdate(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t))

	// When updating with a blank concept
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "", Definition: "Old def."})

	// Then it fails validation and never reaches the repository's Update
	assert.ErrorIs(t, err, domainknowledge.ErrConceptRequired)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateItem_trimsTopicWhitespace_beforePersisting(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Topic == "Distributed systems"
	})).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Distributed systems", "").Return(nil, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, ([]domainknowledge.Chunk)(nil)).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When updating with a topic padded with whitespace
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "  Distributed systems  ", Concept: "New", Definition: "New def.",
	})

	// Then the persisted and returned topic is trimmed
	require.NoError(t, err)
	assert.Equal(t, "Distributed systems", updated.Topic)
}

func TestUpdateItem_returnsTopicRequired_whenTopicIsBlank_andNeverCallsUpdate(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t))

	// When updating with a whitespace-only topic
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "   ", Concept: "New", Definition: "New def."})

	// Then it fails validation and never reaches the repository's Update
	assert.ErrorIs(t, err, domainknowledge.ErrTopicRequired)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateItem_propagatesNotFound_whenItemDoesNotExist(t *testing.T) {
	// Given no matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t))

	// When updating it
	_, err := service.UpdateItem(ctx, "missing", ItemFields{Topic: "Go", Concept: "X", Definition: "Y"})

	// Then the not-found error propagates
	assert.ErrorIs(t, err, domainknowledge.ErrItemNotFound)
}

func TestUpdateItem_returnsErrIndexLoading_whenIndexIsLoading_andNeverTouchesTheRepository(t *testing.T) {
	// Given a loading/retrying index
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(ErrIndexLoading).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard)

	// When updating an item
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "X", Definition: "Y"})

	// Then the mutation is rejected before ever reading the repository
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestUpdateItem_returnsIndexingWarning_whenPostCommitReconciliationFails_butKeepsTheDurableResult(t *testing.T) {
	// Given an existing item whose update persists successfully
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", "").Return(updatedChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t))

	// When updating it and the post-commit reconciliation fails
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "New", Definition: "New def."})

	// Then the durable update is not reported as a failure
	var warning *IndexingWarning
	require.ErrorAs(t, err, &warning)
	assert.ErrorIs(t, warning.Err, boom)
	assert.Equal(t, "item-1", updated.ID)
	assert.Equal(t, "New", updated.Concept)
}
