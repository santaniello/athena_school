package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func duplicateCandidate(topic, concept, definition string) domainknowledge.Item {
	return domainknowledge.Item{Topic: topic, Concept: concept, Definition: definition}
}

func TestFindDuplicates_returnsExactMatch_withoutEmbeddingCall(t *testing.T) {
	// Given an item already saved with the same normalized concept in the
	// same topic, across every lifecycle status
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().
		FindByNormalizedConcept(ctx, "System Design", "cache aside pattern").
		Return([]domainknowledge.Item{
			{ID: "item-approved", Concept: "Cache-Aside Pattern", Status: domainknowledge.StatusApproved},
			{ID: "item-draft", Concept: "cache aside pattern", Status: domainknowledge.StatusDraft},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	store := knowledgemocks.NewMockVectorStore(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", " Cache-Aside  Pattern ", "A caching strategy.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then both matches are returned as exact, score 1, ordered by item ID
	// ascending — llm/store have no .EXPECT() calls set, so no embedding or
	// search ever happened
	require.NoError(t, err)
	require.Equal(t, []domainknowledge.DuplicateMatch{
		{ItemID: "item-approved", Concept: "Cache-Aside Pattern", Status: domainknowledge.StatusApproved, MatchType: domainknowledge.MatchExact, Score: 1},
		{ItemID: "item-draft", Concept: "cache aside pattern", Status: domainknowledge.StatusDraft, MatchType: domainknowledge.MatchExact, Score: 1},
	}, matches)
}

func TestFindDuplicates_returnsNil_whenVectorStoreIsEmpty_withoutEmbeddingCall(t *testing.T) {
	// Given no exact match and an empty vector store
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(0)
	llm := llmmocks.NewMockProvider(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then no matches and no error — llm has no .EXPECT(), so no embedding call happened
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestFindDuplicates_embedsConceptAndDefinition_andSearchesTopicScopedAthenaChunks(t *testing.T) {
	// Given no exact match and a non-empty vector store with one semantic match
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{ID: "item-1", Concept: "Resiliência de Circuito", Status: domainknowledge.StatusApproved}, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().
		Search(ctx, []float32{0.1, 0.2, 0.3}, domainknowledge.DefaultDuplicateTopK,
			domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-1", ItemID: "item-1"}, Score: 0.9375},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1, 0.2, 0.3}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then the semantic match is returned, resolved to its owning item's
	// current concept and status
	require.NoError(t, err)
	require.Equal(t, []domainknowledge.DuplicateMatch{
		{ItemID: "item-1", Concept: "Resiliência de Circuito", Status: domainknowledge.StatusApproved, MatchType: domainknowledge.MatchSemantic, Score: 0.9375},
	}, matches)
}

func TestFindDuplicates_excludesSemanticMatchesBelowThreshold(t *testing.T) {
	// Given a semantic match scoring below the injected threshold
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-1", ItemID: "item-1"}, Score: 0.80},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates with the default 0.90 threshold
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then the below-threshold match is excluded — items.GetByID is never
	// called, since nothing survives the threshold to resolve
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestFindDuplicates_raisingTheThreshold_changesWhichMatchesSurvive(t *testing.T) {
	// Given the exact same 0.80-scoring semantic match as above
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{ID: "item-1", Concept: "Circuit Breaking", Status: domainknowledge.StatusApproved}, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-1", ItemID: "item-1"}, Score: 0.80},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates with a lower, explicitly injected threshold
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, 0.75)

	// Then the same match now survives — the threshold is a parameter, not
	// a hardcoded constant
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "item-1", matches[0].ItemID)
}

func TestFindDuplicates_dedupsMultipleChunksOfTheSameItem_keepingTheHighestScore(t *testing.T) {
	// Given two chunks owned by the same item, the highest-scoring one first
	// (VectorStore.Search's own documented ordering)
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{ID: "item-1", Concept: "Circuit Breaking", Status: domainknowledge.StatusApproved}, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-1", ItemID: "item-1"}, Score: 0.96875},
			{Chunk: domainknowledge.Chunk{ID: "chunk-2", ItemID: "item-1"}, Score: 0.90625},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then item-1 is returned exactly once, at its highest score —
	// items.GetByID is called only once for it (mockery's default
	// expectation count is exactly-once, so a second call would fail)
	require.NoError(t, err)
	require.Equal(t, []domainknowledge.DuplicateMatch{
		{ItemID: "item-1", Concept: "Circuit Breaking", Status: domainknowledge.StatusApproved, MatchType: domainknowledge.MatchSemantic, Score: 0.96875},
	}, matches)
}

func TestFindDuplicates_includesASemanticMatchExactlyAtTheThreshold(t *testing.T) {
	// Given a semantic match scoring exactly at the injected threshold
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{ID: "item-1", Concept: "Circuit Breaking", Status: domainknowledge.StatusApproved}, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-1", ItemID: "item-1"}, Score: 0.9375},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates with minScore exactly equal to the match's score
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, 0.9375)

	// Then "below threshold" excludes it only when strictly lower — a score
	// exactly at the threshold still counts as a match
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "item-1", matches[0].ItemID)
}

func TestFindDuplicates_ordersSemanticMatchesByScoreDescending(t *testing.T) {
	// Given two distinct items scoring differently
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-lower").
		Return(domainknowledge.Item{ID: "item-lower", Concept: "Lower", Status: domainknowledge.StatusApproved}, nil)
	items.EXPECT().GetByID(ctx, "item-higher").
		Return(domainknowledge.Item{ID: "item-higher", Concept: "Higher", Status: domainknowledge.StatusApproved}, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-lower", ItemID: "item-lower"}, Score: 0.90625},
			{Chunk: domainknowledge.Chunk{ID: "chunk-higher", ItemID: "item-higher"}, Score: 0.96875},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then the result is ordered by score descending, regardless of the
	// order VectorStore.Search happened to return them in
	require.NoError(t, err)
	require.Len(t, matches, 2)
	assert.Equal(t, "item-higher", matches[0].ItemID)
	assert.Equal(t, "item-lower", matches[1].ItemID)
}

func TestFindDuplicates_breaksSemanticScoreTiesByItemIDAscending(t *testing.T) {
	// Given two distinct items tied on score
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-b").
		Return(domainknowledge.Item{ID: "item-b", Concept: "B", Status: domainknowledge.StatusApproved}, nil)
	items.EXPECT().GetByID(ctx, "item-a").
		Return(domainknowledge.Item{ID: "item-a", Concept: "A", Status: domainknowledge.StatusApproved}, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-b", ItemID: "item-b"}, Score: 0.9375},
			{Chunk: domainknowledge.Chunk{ID: "chunk-a", ItemID: "item-a"}, Score: 0.9375},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then the tie is broken by item ID ascending, not Search's return order
	require.NoError(t, err)
	require.Len(t, matches, 2)
	assert.Equal(t, "item-a", matches[0].ItemID)
	assert.Equal(t, "item-b", matches[1].ItemID)
}

func TestFindDuplicates_dropsAnOrphanedChunk_whoseOwningItemNoLongerExists(t *testing.T) {
	// Given a semantic match whose owning item was deleted after the chunk
	// was indexed
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return([]domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{ID: "chunk-1", ItemID: "item-1"}, Score: 0.95},
		}, nil)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then the orphaned chunk is silently dropped rather than failing
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestFindDuplicates_returnsErrSemanticDuplicateCheckUnavailable_whenEmbeddingFails(t *testing.T) {
	// Given no exact match, a non-empty store, and a failing embedding call
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{}, errors.New("openrouter: unauthorized"))
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then it fails with the typed, presentable warning — store.Search is
	// never called, since it has no .EXPECT() set
	require.ErrorIs(t, err, domainknowledge.ErrSemanticDuplicateCheckUnavailable)
	require.Empty(t, matches)
}

func TestFindDuplicates_returnsErrSemanticDuplicateCheckUnavailable_whenSearchFails(t *testing.T) {
	// Given no exact match, a non-empty store, and a failing vector search
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").Return(nil, nil)
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Len().Return(3)
	store.EXPECT().Search(ctx, []float32{0.1}, domainknowledge.DefaultDuplicateTopK,
		domainknowledge.SearchFilters{Topic: "System Design", Source: domainknowledge.SourceAthena}).
		Return(nil, errors.New("vectorstore: search failed"))
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Circuit Breaker\n\nA resiliency pattern."}).Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}}, nil)
	service := NewService(items, nil, nil, llm, nil, nil, nil, store, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then it fails with the typed, presentable warning
	require.ErrorIs(t, err, domainknowledge.ErrSemanticDuplicateCheckUnavailable)
	require.Empty(t, matches)
}

func TestFindDuplicates_returnsError_whenExactLookupFails(t *testing.T) {
	// Given a repository failure on the exact-match lookup itself
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "System Design", "circuit breaker").
		Return(nil, errors.New("sqlite: database is locked"))
	llm := llmmocks.NewMockProvider(t)
	service := NewService(items, nil, nil, llm, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding duplicates
	matches, err := service.FindDuplicates(ctx, candidate, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// Then it fails as a genuine error, not the semantic-unavailable
	// warning — llm/store have no .EXPECT() set, so nothing further ran
	require.Error(t, err)
	require.NotErrorIs(t, err, domainknowledge.ErrSemanticDuplicateCheckUnavailable)
	require.Empty(t, matches)
}
