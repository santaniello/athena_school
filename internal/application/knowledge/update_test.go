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
	"github.com/santaniello/athena/internal/infrastructure/vectorstore"
)

func TestUpdateItem_reindexesTheChunk_whenConceptOrDefinitionChanges(t *testing.T) {
	// Given an existing approved item, edited with a genuinely new
	// Concept/Definition — content the chunk is rendered from
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
	chunks := knowledgemocks.NewMockChunkRepository(t)
	// First DeleteByItemID is UpdateItem's own eviction, inside its
	// transaction; the second is indexKnowledgeItem's own delete-then-insert,
	// which finds nothing left.
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "New\n\nNew def."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1" && cs[0].Content == "New\n\nNew def."
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(nil).Once()
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating its concept and definition
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "Go", Concept: "New", Definition: "New def.",
	})

	// Then the change persists, the stale chunk is evicted, and a fresh one is embedded
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Concept)
	assert.Equal(t, domainknowledge.StatusApproved, updated.Status)
	assert.Equal(t, domainknowledge.SourceAthena, updated.Source)
	assert.True(t, updated.CreatedAt.Equal(originalCreatedAt))
}

func TestUpdateItem_reindexesTheChunk_whenOnlyPropertiesChange(t *testing.T) {
	// Given an existing item whose Concept/Definition stay the same but
	// whose Properties differ — still part of the rendered chunk content
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Properties: []string{"typed"}, Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Concept == "Channels" && item.Definition == "Typed conduits." &&
			assert.ObjectsAreEqual([]string{"typed", "blocking"}, item.Properties) &&
			item.Status == domainknowledge.StatusApproved && item.Source == domainknowledge.SourceAthena
	})).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Times(2)
	wantContent := "Channels\n\nTyped conduits.\n\nProperties:\n- typed\n- blocking"
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: wantContent}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1" && cs[0].Content == wantContent
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string(nil)).Return(nil).Times(2)
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating with a new Properties list, same Concept/Definition
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Properties: []string{"typed", "blocking"},
	})

	// Then it still re-embeds rather than treating this as a metadata-only edit
	require.NoError(t, err)
	assert.Equal(t, "Channels", updated.Concept)
}

func TestUpdateItem_staysOnTheMetadataOnlyPath_whenOnlyTopicChanges(t *testing.T) {
	// Given an existing item whose Concept/Definition/Properties/TradeOffs
	// are resubmitted unchanged — only Topic differs
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Properties: []string{"typed"}, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Topic == "Distributed systems"
	})).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1", Topic: "Distributed systems"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Distributed systems", domainknowledge.StatusApproved,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(updatedChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating with only the topic changed (padded with whitespace)
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "  Distributed systems  ", Concept: "Channels", Definition: "Typed conduits.",
		Properties: []string{"typed"},
	})

	// Then no embedding call is made and the persisted topic is trimmed
	require.NoError(t, err)
	assert.Equal(t, "Distributed systems", updated.Topic)
	llm.AssertNotCalled(t, "Embeddings", mock.Anything, mock.Anything)
	chunks.AssertNotCalled(t, "DeleteByItemID", mock.Anything, mock.Anything)
}

func TestUpdateItem_staysOnTheMetadataOnlyPath_whenPropertiesGoFromNilToAnEmptySlice(t *testing.T) {
	// Given an existing item with no Properties/TradeOffs at all (nil) and
	// a resubmission with an empty slice instead — renderItemContent treats
	// both identically, so this must not be seen as a content change
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1", Topic: "Go"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Go", domainknowledge.StatusApproved,
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(updatedChunks, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating with Properties/TradeOffs submitted as empty slices
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Properties: []string{}, TradeOffs: []string{},
	})

	// Then no embedding call is made — a nil-to-empty-slice edit renders
	// identical content and stays on the cheaper metadata-only path
	require.NoError(t, err)
	assert.Equal(t, "item-1", updated.ID)
	llm.AssertNotCalled(t, "Embeddings", mock.Anything, mock.Anything)
	chunks.AssertNotCalled(t, "DeleteByItemID", mock.Anything, mock.Anything)
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
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating with a blank concept
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "", Definition: "Old def."})

	// Then it fails validation and never reaches the repository's Update
	assert.ErrorIs(t, err, domainknowledge.ErrConceptRequired)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
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
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

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
	service := NewService(repository, nil, nil, nil, nil, nil, tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

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
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, guard, domainknowledge.RetrievalThresholds{})

	// When updating an item
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "X", Definition: "Y"})

	// Then the mutation is rejected before ever reading the repository
	assert.ErrorIs(t, err, ErrIndexLoading)
	repository.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestUpdateItem_returnsIndexingWarning_whenPostCommitReconciliationFails_onTheMetadataOnlyPath(t *testing.T) {
	// Given a topic-only edit (metadata-only path) whose store reconciliation fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	updatedChunks := []domainknowledge.Chunk{{ID: "chunk-1", ItemID: "item-1"}}
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().UpdateMetadataByItemID(ctx, "item-1", "Distributed systems", "",
		mock.MatchedBy(func(ts time.Time) bool { return !ts.IsZero() })).Return(updatedChunks, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Add(mock.Anything, updatedChunks).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating it (topic only) and the post-commit reconciliation fails
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "Distributed systems", Concept: "Channels", Definition: "Typed conduits.",
	})

	// Then the durable update is not reported as a failure
	assert.ErrorIs(t, err, ErrIndexingFailed)
	assert.Equal(t, "item-1", updated.ID)
	assert.Equal(t, "Distributed systems", updated.Topic)
}

func TestUpdateItem_returnsErrIndexingFailed_whenReindexingFails_afterEvictingTheStaleChunk(t *testing.T) {
	// Given a content edit whose old chunk is evicted successfully but
	// whose re-embed then fails (e.g. the OpenRouter key is missing)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	boom := errors.New("openrouter unavailable")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "New\n\nNew def."}).Return(domainllm.EmbeddingResponse{}, boom).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating it and the re-embed fails
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "New", Definition: "New def."})

	// Then the durable edit persists, the stale chunk is already evicted
	// (so search returns nothing rather than an obsolete definition), and
	// the failure wraps ErrIndexingFailed
	assert.ErrorIs(t, err, ErrIndexingFailed)
	assert.Equal(t, "New", updated.Concept)
}

func TestUpdateItem_returnsErrIndexingFailed_whenEvictingTheStaleChunkFails(t *testing.T) {
	// Given a content edit whose SQLite-level eviction commits but whose
	// in-memory VectorStore eviction fails
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(boom).Once()
	llm := llmmocks.NewMockProvider(t)
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating it and eviction itself fails
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "New", Definition: "New def."})

	// Then the durable edit persists, no embedding is even attempted, and
	// the failure wraps ErrIndexingFailed
	assert.ErrorIs(t, err, ErrIndexingFailed)
	assert.Equal(t, "New", updated.Concept)
	llm.AssertNotCalled(t, "Embeddings", mock.Anything, mock.Anything)
}

func TestUpdateItem_selfHealsTheOrphanedChunk_whenLaterReindexed(t *testing.T) {
	// Given the same failure as above — the stale chunk survives in the
	// VectorStore because Remove failed — but exercised against the real
	// vectorstore.Store instead of a mock, so its Add can actually run its
	// by-ItemID eviction (see
	// specs/phases/phase-02-knowledge-engine/08-01-vectorstore-orphan-chunk-recovery.md,
	// Option B)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	// An empty ID makes the real Store.Remove fail, the same shape a real
	// eviction failure takes, without a mock standing in for VectorStore.
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{""}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	store := vectorstore.New()
	require.NoError(t, store.ReplaceAll(ctx, []domainknowledge.Chunk{{
		ID: "chunk-1", ItemID: "item-1", Source: domainknowledge.SourceAthena,
		Topic: "Go", Status: domainknowledge.StatusApproved, Embedding: []float32{1, 0},
	}}))
	service := NewService(repository, nil, nil, llm, nil, chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})

	// When updating it and eviction itself fails
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "New", Definition: "New def."})
	require.ErrorIs(t, err, ErrIndexingFailed)
	require.Equal(t, 1, store.Len(), "the orphaned chunk-1 is still sitting in the store")

	// And when the item is later reindexed again — e.g. by the backfill
	// sweep, which is exactly what ListUnindexed picks it up for once it
	// has zero current SQLite chunks
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1" && cs[0].ID != "chunk-1"
	})).Return(nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "New\n\nNew def."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0, 1}}, nil).Once()

	require.NoError(t, service.indexKnowledgeItem(ctx, updated))

	// Then the store converges to exactly one chunk for the item — the
	// orphan is gone, not duplicated alongside the fresh one
	assert.Equal(t, 1, store.Len())
}
