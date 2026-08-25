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
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func TestCountUnindexedItems_delegatesToTheRepositoryWithTheCurrentEmbeddingModel(t *testing.T) {
	// Given a repository reporting 3 unindexed items
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().CountUnindexed(ctx, domainllm.EmbeddingModel).Return(3, nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{})

	// When counting unindexed items
	count, err := service.CountUnindexedItems(ctx)

	// Then the repository's count is returned as-is
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestReindexKnowledgeItems_indexesEveryItem_andReportsProgressForEach(t *testing.T) {
	// Given two unindexed items, both of which will embed successfully
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	items := []domainknowledge.Item{
		{ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.", Status: domainknowledge.StatusApproved},
		{ID: "item-2", Topic: "Rust", Concept: "Borrowing", Definition: "Ownership rules.", Status: domainknowledge.StatusDraft},
	}
	repository.EXPECT().ListUnindexed(ctx, domainllm.EmbeddingModel).Return(items, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Times(2)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, mock.Anything).Return(nil, nil).Times(2)
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Times(2)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Times(2)
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Times(2)
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	var progressCalls []ReindexProgress
	onProgress := func(p ReindexProgress) error {
		progressCalls = append(progressCalls, p)
		return nil
	}

	// When reindexing the backlog
	summary, err := service.ReindexKnowledgeItems(ctx, onProgress)

	// Then every item is indexed and progress is reported once per item
	require.NoError(t, err)
	assert.Equal(t, ReindexSummary{ItemsProcessed: 2, ItemsIndexed: 2}, summary)
	require.Len(t, progressCalls, 2)
	assert.Equal(t, ReindexProgress{ItemsProcessed: 1, ItemsTotal: 2, CurrentTopic: "Go"}, progressCalls[0])
	assert.Equal(t, ReindexProgress{ItemsProcessed: 2, ItemsTotal: 2, CurrentTopic: "Rust"}, progressCalls[1])
}

func TestReindexKnowledgeItems_continuesPastAFailure_ratherThanStoppingEarly(t *testing.T) {
	// Given two unindexed items where the first fails to embed — unlike
	// SaveDrafts, this explicit backfill run must still attempt the second
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	items := []domainknowledge.Item{
		{ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
		{ID: "item-2", Topic: "Rust", Concept: "Borrowing", Definition: "Ownership rules."},
	}
	repository.EXPECT().ListUnindexed(ctx, domainllm.EmbeddingModel).Return(items, nil).Once()
	boom := errors.New("openrouter unavailable")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(domainllm.EmbeddingResponse{}, boom).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-2").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When reindexing the backlog
	summary, err := service.ReindexKnowledgeItems(ctx, nil)

	// Then both items are attempted: one failure, one success
	require.NoError(t, err)
	assert.Equal(t, 2, summary.ItemsProcessed)
	assert.Equal(t, 1, summary.ItemsIndexed)
	assert.Equal(t, 1, summary.ItemsFailed)
	require.Len(t, summary.Failures, 1)
	assert.Equal(t, "item-1", summary.Failures[0].ItemID)
	assert.Contains(t, summary.Failures[0].Reason, boom.Error())
}

func TestReindexKnowledgeItems_stopsImmediately_whenOnProgressReturnsAnError(t *testing.T) {
	// Given two unindexed items and a caller that aborts after the first
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	items := []domainknowledge.Item{
		{ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
		{ID: "item-2", Topic: "Rust", Concept: "Borrowing", Definition: "Ownership rules."},
	}
	repository.EXPECT().ListUnindexed(ctx, domainllm.EmbeddingModel).Return(items, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	abortErr := errors.New("dialog closed")
	onProgress := func(ReindexProgress) error { return abortErr }

	// When reindexing and the progress callback aborts
	summary, err := service.ReindexKnowledgeItems(ctx, onProgress)

	// Then the run stops immediately, having processed only the first item
	assert.ErrorIs(t, err, abortErr)
	assert.Equal(t, 1, summary.ItemsProcessed)
}

func TestReindexKnowledgeItems_propagatesTheListError(t *testing.T) {
	// Given a repository whose listing fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	listErr := errors.New("database locked")
	repository.EXPECT().ListUnindexed(ctx, domainllm.EmbeddingModel).Return(nil, listErr).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When reindexing the backlog
	_, err := service.ReindexKnowledgeItems(ctx, nil)

	// Then the error propagates
	assert.ErrorIs(t, err, listErr)
}

func TestReindexKnowledgeItems_returnsErrIndexLoading_whenIndexIsLoading_andNeverListsItems(t *testing.T) {
	// Given a loading/retrying index
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(ErrIndexLoading).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard, domainknowledge.RetrievalThresholds{})

	// When reindexing the backlog
	_, err := service.ReindexKnowledgeItems(ctx, nil)

	// Then the mutation is rejected before ever listing items
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "ListUnindexed", mock.Anything, mock.Anything)
}
