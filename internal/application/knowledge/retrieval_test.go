package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

// readyGuard returns an IndexGuard mock reporting a valid, non-empty
// snapshot — the default for every test that isn't specifically about
// index readiness.
func readyGuard(t *testing.T) *txmocks.MockIndexGuard {
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().Status().Return(domainknowledge.IndexStatus{State: domainknowledge.IndexStateReady, HasSnapshot: true})
	return guard
}

func defaultThresholds(t *testing.T) domainknowledge.RetrievalThresholds {
	t.Helper()
	thresholds, err := domainknowledge.NewRetrievalThresholds(domainknowledge.DefaultMinSimilarity, domainknowledge.DefaultSufficiency)
	require.NoError(t, err)
	return thresholds
}

// cleanThresholds builds thresholds from values exactly representable in
// both float32 and float64 (halves, quarters), so a boundary test's score
// round-trips through float32 without the precision loss 0.35/0.55 would
// introduce.
func cleanThresholds(t *testing.T, minScore, sufficiency float64) domainknowledge.RetrievalThresholds {
	t.Helper()
	thresholds, err := domainknowledge.NewRetrievalThresholds(minScore, sufficiency)
	require.NoError(t, err)
	return thresholds
}

func scoredChunk(id, itemID string, score float32) domainknowledge.ScoredChunk {
	return domainknowledge.ScoredChunk{
		Chunk: domainknowledge.Chunk{
			ID: id, ItemID: itemID, Source: domainknowledge.SourceImportedDoc,
			Topic: "Go", Status: domainknowledge.StatusApproved,
			FilePath: "notes/" + id + ".md", Heading: "Heading " + id, Content: "Content " + id,
		},
		Score: score,
	}
}

func TestRetrieve_returnsErrVectorStoreUnavailable_whenNoSnapshot(t *testing.T) {
	// Given an index that has never loaded a snapshot
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().Status().Return(domainknowledge.IndexStatus{HasSnapshot: false})
	store := knowledgemocks.NewMockVectorStore(t)
	llm := llmmocks.NewMockProvider(t)
	items := knowledgemocks.NewMockRepository(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	_, err := service.Retrieve(context.Background(), "session-1", "Topic: Go\n\nMessage: what is a channel?")

	// Then it fails with ErrVectorStoreUnavailable; store/llm/items have no
	// .EXPECT() set, so an unexpected call would fail the test
	require.ErrorIs(t, err, domainknowledge.ErrVectorStoreUnavailable)
}

func TestRetrieve_returnsEmptyResult_whenSnapshotValidButStoreEmpty(t *testing.T) {
	// Given a valid but empty snapshot
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0)
	llm := llmmocks.NewMockProvider(t)
	items := knowledgemocks.NewMockRepository(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "Topic: Go\n\nMessage: what is a channel?")

	// Then it returns a zero-value result with no error; llm has no
	// .EXPECT() set, so no embedding call happened
	require.NoError(t, err)
	require.Equal(t, domainknowledge.RetrievalResult{}, result)
}

func TestRetrieve_embedsQueryWithSessionAttribution_andSearchesApprovedOnlyWithDefaultTopK(t *testing.T) {
	// Given a valid, non-empty snapshot with one matching chunk
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().
		Search(context.Background(), []float32{0.1, 0.2, 0.3}, domainknowledge.DefaultTopK, domainknowledge.SearchFilters{Status: domainknowledge.StatusApproved}).
		Return([]domainknowledge.ScoredChunk{scoredChunk("chunk-1", "item-1", 0.9)}, nil).
		Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "session-1", Input: "Topic: Go\n\nMessage: what is a channel?"}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1, 0.2, 0.3}}, nil).
		Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(context.Background(), "item-1").Return(domainknowledge.Item{ID: "item-1", Concept: "Channels"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "Topic: Go\n\nMessage: what is a channel?")

	// Then the single chunk survives into Chunks and Sources
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
	require.Len(t, result.Sources, 1)
	require.Equal(t, "chunk-1", result.Sources[0].ChunkID)
	require.Equal(t, "Channels", result.Sources[0].Concept)
	require.NotEmpty(t, result.Context)
}

func TestRetrieve_filtersChunksBelowMinScore(t *testing.T) {
	// Given one chunk above minScore and one below it
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(2)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{
			scoredChunk("chunk-high", "item-1", 0.9),
			scoredChunk("chunk-low", "item-2", 0.10),
		}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(context.Background(), "item-1").Return(domainknowledge.Item{ID: "item-1", Concept: "Channels"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then only the chunk scoring at or above minScore survives; items is
	// never asked to resolve item-2 (no .EXPECT() for it)
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
	require.Equal(t, "chunk-high", result.Chunks[0].Chunk.ID)
}

func TestRetrieve_includesChunkAtExactlyMinScore(t *testing.T) {
	// Given a chunk scoring exactly at minScore
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{scoredChunk("chunk-1", "item-1", 0.5)}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{ID: "item-1", Concept: "Channels"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, cleanThresholds(t, 0.5, 0.9))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then equality is inclusive — the chunk survives
	require.NoError(t, err)
	require.Len(t, result.Chunks, 1)
}

func TestRetrieve_resolvesConceptOncePerDistinctItemID(t *testing.T) {
	// Given two surviving chunks sharing one owning item
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(2)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{
			scoredChunk("chunk-1", "item-1", 0.9),
			scoredChunk("chunk-2", "item-1", 0.8),
		}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{ID: "item-1", Concept: "Channels"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then both chunks survive as separate Sources, each carrying the same
	// concept — items.GetByID was called only Once (see .EXPECT() above)
	require.NoError(t, err)
	require.Len(t, result.Sources, 2)
	require.Equal(t, "Channels", result.Sources[0].Concept)
	require.Equal(t, "Channels", result.Sources[1].Concept)
}

func TestRetrieve_returnsIntegrityError_whenOwningItemMissing(t *testing.T) {
	// Given a surviving chunk whose owning item does not exist
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{scoredChunk("chunk-1", "item-missing", 0.9)}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	_, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then it fails as an integrity error, wrapping the domain sentinel
	require.ErrorIs(t, err, domainknowledge.ErrItemNotFound)
}

func TestRetrieve_preservesScoreDescendingIDAscendingOrder_acrossChunksSourcesAndJSON(t *testing.T) {
	// Given three chunks already in Search's documented order
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{
			scoredChunk("chunk-a", "item-1", 0.9),
			scoredChunk("chunk-b", "item-2", 0.8),
			scoredChunk("chunk-c", "item-3", 0.7),
		}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, mock.Anything).Return(domainknowledge.Item{Concept: "C"}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")
	require.NoError(t, err)

	// Then Chunks, Sources, and the JSON entries all preserve the same order
	require.Equal(t, []string{"chunk-a", "chunk-b", "chunk-c"}, chunkIDs(result.Chunks))
	require.Equal(t, []string{"chunk-a", "chunk-b", "chunk-c"}, sourceChunkIDs(result.Sources))

	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Context), &entries))
	require.Equal(t, []string{"Heading chunk-a", "Heading chunk-b", "Heading chunk-c"}, headings(entries))
}

func TestRetrieve_excludesEmbeddingAndScoreFromRenderedJSON(t *testing.T) {
	// Given one surviving chunk
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{scoredChunk("chunk-1", "item-1", 0.9)}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{Concept: "Channels"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")
	require.NoError(t, err)

	// Then each JSON entry has exactly the five documented keys
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Context), &entries))
	require.Len(t, entries, 1)
	require.ElementsMatch(t, []string{"sourceType", "filePath", "heading", "concept", "content"}, keysOf(entries[0]))
}

func TestRetrieve_capsContext_removingLowestScoreChunksWholeUntilUnderBudget(t *testing.T) {
	// Given two chunks whose combined JSON exceeds the 8000-code-point cap,
	// the lowest-scoring one alone being large enough to push it over
	guard := readyGuard(t)
	big := scoredChunk("chunk-big", "item-1", 0.6)
	big.Chunk.Content = strings.Repeat("x", 7900)
	small := scoredChunk("chunk-small", "item-2", 0.95)
	small.Chunk.Content = "short"
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(2)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{small, big}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, mock.Anything).Return(domainknowledge.Item{Concept: "C"}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")
	require.NoError(t, err)

	// Then the lowest-scoring chunk (chunk-big) is dropped whole; the
	// higher-scoring chunk-small remains, with its content untruncated
	require.Len(t, result.Chunks, 1)
	require.Equal(t, "chunk-small", result.Chunks[0].Chunk.ID)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Context), &entries))
	require.Equal(t, "short", entries[0]["content"])
}

func TestRetrieve_dropsSoleOversizedChunk_yieldingNoMatch(t *testing.T) {
	// Given a single surviving chunk whose content alone exceeds the cap
	guard := readyGuard(t)
	oversized := scoredChunk("chunk-1", "item-1", 0.9)
	oversized.Chunk.Content = strings.Repeat("x", maxContextChars+1)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{oversized}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{Concept: "C"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then the result is the same as no local match
	require.NoError(t, err)
	require.Equal(t, domainknowledge.RetrievalResult{}, result)
}

func TestRetrieve_includesChunkAtExactlyTheCapBoundary(t *testing.T) {
	// Given a single surviving chunk whose rendered JSON lands at exactly
	// maxContextChars code points (computed via the real rendering function
	// with empty content, then padded to hit the boundary precisely)
	empty := scoredChunk("chunk-1", "item-1", 0.9)
	empty.Chunk.Content = ""
	emptyRendered, err := renderContext([]domainknowledge.ScoredChunk{empty}, map[string]string{"item-1": "C"})
	require.NoError(t, err)
	paddingLen := maxContextChars - utf8.RuneCountInString(emptyRendered)
	require.Positive(t, paddingLen)

	atBoundary := scoredChunk("chunk-1", "item-1", 0.9)
	atBoundary.Chunk.Content = strings.Repeat("x", paddingLen)
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{atBoundary}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{Concept: "C"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, retrieveErr := service.Retrieve(context.Background(), "session-1", "query")

	// Then a JSON block landing exactly on the cap is included, not dropped
	require.NoError(t, retrieveErr)
	require.Len(t, result.Chunks, 1)
	require.Equal(t, maxContextChars, utf8.RuneCountInString(result.Context))
}

func TestRetrieve_dropsSoleChunk_whenRenderedJSONIsOneCodePointOverTheCap(t *testing.T) {
	// Given a single surviving chunk whose rendered JSON lands exactly one
	// code point over maxContextChars
	empty := scoredChunk("chunk-1", "item-1", 0.9)
	empty.Chunk.Content = ""
	emptyRendered, err := renderContext([]domainknowledge.ScoredChunk{empty}, map[string]string{"item-1": "C"})
	require.NoError(t, err)
	paddingLen := maxContextChars - utf8.RuneCountInString(emptyRendered) + 1
	require.Positive(t, paddingLen)

	overBoundary := scoredChunk("chunk-1", "item-1", 0.9)
	overBoundary.Chunk.Content = strings.Repeat("x", paddingLen)
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{overBoundary}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{Concept: "C"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	result, retrieveErr := service.Retrieve(context.Background(), "session-1", "query")

	// Then it is dropped whole, same as no local match
	require.NoError(t, retrieveErr)
	require.Equal(t, domainknowledge.RetrievalResult{}, result)
}

func TestRetrieve_sufficientIsBasedOnlyOnPostCapSurvivors(t *testing.T) {
	// Given the only chunk meeting the sufficiency threshold is the lowest
	// scoring one, and it gets capped away for being oversized
	guard := readyGuard(t)
	sufficientButOversized := scoredChunk("chunk-big", "item-1", 0.75)
	sufficientButOversized.Chunk.Content = strings.Repeat("x", 7900)
	insufficientButSmall := scoredChunk("chunk-small", "item-2", 0.5)
	insufficientButSmall.Chunk.Content = "short"
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(2)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{insufficientButSmall, sufficientButOversized}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, mock.Anything).Return(domainknowledge.Item{Concept: "C"}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, cleanThresholds(t, 0.5, 0.75))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")
	require.NoError(t, err)

	// Then the surviving (capped-in) chunk scores below sufficiency, so the
	// result as a whole is not Sufficient — a result discarded by the cap
	// cannot make the context sufficient
	require.Len(t, result.Chunks, 1)
	require.Equal(t, "chunk-small", result.Chunks[0].Chunk.ID)
	require.False(t, result.Sufficient)
}

func TestRetrieve_sufficientTrue_whenAnyPostCapSurvivorMeetsOrExceedsThreshold(t *testing.T) {
	// Given a surviving chunk scoring exactly at the sufficiency threshold
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domainknowledge.ScoredChunk{scoredChunk("chunk-1", "item-1", 0.75)}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{Concept: "C"}, nil).Once()
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, cleanThresholds(t, 0.5, 0.75))

	// When retrieving
	result, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then equality counts as sufficient
	require.NoError(t, err)
	require.True(t, result.Sufficient)
}

func TestRetrieve_propagatesEmbeddingsError(t *testing.T) {
	// Given an LLM whose embedding call fails
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	llm := llmmocks.NewMockProvider(t)
	embedErr := errors.New("embedding provider unavailable")
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{}, embedErr).Once()
	items := knowledgemocks.NewMockRepository(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	_, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then the error propagates; store.Search is never called (no .EXPECT())
	require.ErrorIs(t, err, embedErr)
}

func TestRetrieve_propagatesSearchError(t *testing.T) {
	// Given a vector store whose Search call fails
	guard := readyGuard(t)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(1)
	searchErr := errors.New("vector store index corrupted")
	store.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, searchErr).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(mock.Anything, mock.Anything).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, guard, defaultThresholds(t))

	// When retrieving
	_, err := service.Retrieve(context.Background(), "session-1", "query")

	// Then the error propagates
	require.ErrorIs(t, err, searchErr)
}

func chunkIDs(chunks []domainknowledge.ScoredChunk) []string {
	ids := make([]string, len(chunks))
	for i, c := range chunks {
		ids[i] = c.Chunk.ID
	}
	return ids
}

func sourceChunkIDs(sources []domainknowledge.Source) []string {
	ids := make([]string, len(sources))
	for i, s := range sources {
		ids[i] = s.ChunkID
	}
	return ids
}

func headings(entries []map[string]any) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e["heading"].(string)
	}
	return out
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
