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

func testItemForIndexing() domainknowledge.Item {
	updatedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Properties: []string{"typed", "blocking"}, TradeOffs: []string{"coordination overhead"},
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusDraft,
		UpdatedAt: updatedAt,
	}
}

func TestIndexKnowledgeItem_embedsRendersAndSavesOneChunk_thenAddsItToTheStore(t *testing.T) {
	// Given an item with no existing chunk
	ctx := context.Background()
	item := testItemForIndexing()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{
		Input: "Channels\n\nTyped conduits.\n\nProperties:\n- typed\n- blocking\n\nTrade-offs:\n- coordination overhead",
	}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1, 0.2}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 &&
			cs[0].ID != "" &&
			cs[0].Source == domainknowledge.SourceAthena &&
			cs[0].Topic == "Go" &&
			cs[0].Status == domainknowledge.StatusDraft &&
			cs[0].ItemID == "item-1" &&
			cs[0].EmbeddingModel == domainllm.EmbeddingModel &&
			cs[0].ItemUpdatedAt.Equal(item.UpdatedAt) &&
			len(cs[0].Embedding) == 2
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(nil, nil, nil, llm, nil, chunks, tx, store, nil, domainknowledge.RetrievalThresholds{})

	// When indexing it
	err := service.indexKnowledgeItem(ctx, item)

	// Then it succeeds
	require.NoError(t, err)
}

func TestIndexKnowledgeItem_deletesAndEvictsExistingChunk_whenOneAlreadyExists(t *testing.T) {
	// Given an item that already owns a chunk
	ctx := context.Background()
	item := testItemForIndexing()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"old-chunk"}, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"old-chunk"}).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(nil, nil, nil, llm, nil, chunks, tx, store, nil, domainknowledge.RetrievalThresholds{})

	// When re-indexing it
	err := service.indexKnowledgeItem(ctx, item)

	// Then the old chunk is deleted from SQLite and evicted from the store
	require.NoError(t, err)
}

func TestIndexKnowledgeItem_returnsErrIndexingFailed_whenEmbeddingFails_andNeverTouchesPersistence(t *testing.T) {
	// Given an embedding call that fails (e.g. missing API key)
	ctx := context.Background()
	item := testItemForIndexing()
	boom := errors.New("openrouter unavailable")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(domainllm.EmbeddingResponse{}, boom).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	service := NewService(nil, nil, nil, llm, nil, chunks, nil, nil, nil, domainknowledge.RetrievalThresholds{})

	// When indexing it
	err := service.indexKnowledgeItem(ctx, item)

	// Then it wraps ErrIndexingFailed and never touches the chunk repository
	assert.ErrorIs(t, err, ErrIndexingFailed)
	chunks.AssertNotCalled(t, "DeleteByItemID", mock.Anything, mock.Anything)
}

func TestIndexKnowledgeItem_returnsErrIndexingFailed_whenPersistingTheChunkFails(t *testing.T) {
	// Given an embedding that succeeds but a chunk write that fails
	ctx := context.Background()
	item := testItemForIndexing()
	boom := errors.New("disk full")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(nil, nil, nil, llm, nil, chunks, tx, nil, nil, domainknowledge.RetrievalThresholds{})

	// When indexing it
	err := service.indexKnowledgeItem(ctx, item)

	// Then it wraps ErrIndexingFailed
	assert.ErrorIs(t, err, ErrIndexingFailed)
}

func TestIndexKnowledgeItem_returnsErrIndexingFailed_whenStoreReconciliationFails(t *testing.T) {
	// Given a chunk that persists successfully but fails to reach the
	// in-memory store — SQLite already has it, so a restart self-heals
	ctx := context.Background()
	item := testItemForIndexing()
	boom := errors.New("store exploded")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(nil, nil, nil, llm, nil, chunks, tx, store, nil, domainknowledge.RetrievalThresholds{})

	// When indexing it
	err := service.indexKnowledgeItem(ctx, item)

	// Then it still wraps ErrIndexingFailed
	assert.ErrorIs(t, err, ErrIndexingFailed)
}
