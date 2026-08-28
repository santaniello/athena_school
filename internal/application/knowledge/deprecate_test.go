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
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
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
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(updatedChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

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
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

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
	guard.EXPECT().BeginMutation().Return(ErrIndexLoading).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard, domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deprecating an item
	_, err := service.Deprecate(ctx, "item-1")

	// Then the mutation is rejected before ever reading the repository
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestDeprecate_reindexes_whenTheItemOwnsNoChunkYet(t *testing.T) {
	// Given an approved item with no existing chunk
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(nil, nil).Once()
	wantContent := "Channels\n\nTyped conduits."
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: wantContent}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1" && cs[0].Content == wantContent
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deprecating it
	updated, err := service.Deprecate(ctx, "item-1")

	// Then it recovers by fully re-indexing rather than reconciling nothing
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusDeprecated, updated.Status)
}

func TestDeprecate_returnsErrIndexingFailed_whenRecoveryReindexingFails_butKeepsTheDurableTransition(t *testing.T) {
	// Given an approved item with no existing chunk, and an embedding call that fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(nil, nil).Once()
	boom := errors.New("openrouter unavailable")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Channels\n\nTyped conduits."}).Return(domainllm.EmbeddingResponse{}, boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deprecating it and the recovery re-indexing fails
	updated, err := service.Deprecate(ctx, "item-1")

	// Then the durable transition is preserved and the failure wraps ErrIndexingFailed
	assert.ErrorIs(t, err, ErrIndexingFailed)
	assert.Equal(t, domainknowledge.StatusDeprecated, updated.Status)
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
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusDeprecated,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(updatedChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When deprecating it and the post-commit reconciliation fails
	updated, err := service.Deprecate(ctx, "item-1")

	// Then the durable transition is not reported as a failure
	var warning *IndexingWarning
	require.ErrorAs(t, err, &warning)
	assert.ErrorIs(t, warning.Err, boom)
	assert.Equal(t, "item-1", updated.ID)
	assert.Equal(t, domainknowledge.StatusDeprecated, updated.Status)
}
