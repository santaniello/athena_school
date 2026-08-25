package vectorstore

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

func testChunk(id string, embedding []float32) knowledge.Chunk {
	return knowledge.Chunk{
		ID:        id,
		Source:    knowledge.SourceImportedDoc,
		Topic:     "Go",
		Status:    knowledge.StatusApproved,
		ItemID:    "item-" + id,
		Embedding: embedding,
	}
}

func TestStore_NewStore_startsUnavailable(t *testing.T) {
	// Given a freshly constructed store
	store := New()
	ctx := context.Background()

	// When searching before any ReplaceAll ever ran
	_, err := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})

	// Then it reports unavailable, not an empty result, and Len is zero
	assert.ErrorIs(t, err, knowledge.ErrVectorStoreUnavailable)
	assert.Equal(t, 0, store.Len())
}

func TestStore_Add_doesNotMarkStoreReady(t *testing.T) {
	// Given a fresh store
	store := New()
	ctx := context.Background()

	// When only Add has ever been called (never ReplaceAll)
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))

	// Then Search still reports unavailable — Add alone never establishes readiness
	_, err := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})
	assert.ErrorIs(t, err, knowledge.ErrVectorStoreUnavailable)
}

func TestStore_Add_upsertsByID_withoutIncreasingLen(t *testing.T) {
	// Given a store with one chunk added
	store := New()
	ctx := context.Background()
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))
	require.Equal(t, 1, store.Len())

	// When adding a chunk with the same ID again, in a separate call
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{testChunk("c1", []float32{0, 1})}))

	// Then it replaces the existing entry instead of growing the store
	assert.Equal(t, 1, store.Len())
}

func TestStore_Add_isIdempotent_whenRepeatingTheSameCall(t *testing.T) {
	// Given a chunk
	store := New()
	ctx := context.Background()
	chunk := testChunk("c1", []float32{1, 0})

	// When adding it twice, identically
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{chunk}))
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{chunk}))

	// Then the store still has exactly one entry
	assert.Equal(t, 1, store.Len())
}

func TestStore_Add_rejectsEntireBatch_onOneInvalidChunk(t *testing.T) {
	// Given a batch with one valid chunk and one with an empty embedding
	store := New()
	ctx := context.Background()
	batch := []knowledge.Chunk{
		testChunk("c1", []float32{1, 0}),
		testChunk("c2", nil),
	}

	// When adding the batch
	err := store.Add(ctx, batch)

	// Then the whole batch is rejected and nothing is applied
	assert.ErrorIs(t, err, knowledge.ErrInvalidVector)
	assert.Equal(t, 0, store.Len())
}

func TestStore_Add_rejectsDuplicateIDsWithinOneBatch_andAppliesNothing(t *testing.T) {
	// Given a batch with the same ID twice
	store := New()
	ctx := context.Background()
	batch := []knowledge.Chunk{
		testChunk("c1", []float32{1, 0}),
		testChunk("c1", []float32{0, 1}),
	}

	// When adding the batch
	err := store.Add(ctx, batch)

	// Then it is rejected as a duplicate and nothing is applied
	assert.ErrorIs(t, err, knowledge.ErrDuplicateChunkID)
	assert.Equal(t, 0, store.Len())
}

func TestStore_Add_isSuccessfulNoOp_whenEmpty_evenWithCanceledContext(t *testing.T) {
	// Given a canceled context
	store := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When adding an empty batch
	err := store.Add(ctx, nil)

	// Then it still succeeds
	assert.NoError(t, err)
}

func TestStore_Add_returnsContextError_whenCanceled_andAppliesNoPartialMutation(t *testing.T) {
	// Given a canceled context and a non-empty, otherwise-valid batch
	store := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When adding it
	err := store.Add(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})})

	// Then the context error is returned and nothing is applied
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, store.Len())
}

func TestStore_Add_deepCopiesAndNormalizesEmbedding_withoutMutatingCallersSlice(t *testing.T) {
	// Given a chunk with a non-unit embedding
	store := New()
	ctx := context.Background()
	callerVector := []float32{3, 4}
	chunk := testChunk("c1", callerVector)

	// When adding it
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{chunk}))

	// Then the caller's own slice is untouched
	assert.Equal(t, []float32{3, 4}, callerVector)

	// And the stored/returned embedding is normalized to unit length, not
	// the raw provider vector, and is not backed by the caller's array
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{chunk}))
	results, err := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.InDelta(t, 1.0, vectorNorm(results[0].Chunk.Embedding), 1e-6)
}

func vectorNorm(vec []float32) float64 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum)
}

func TestStore_Add_evictsStaleSiblingSharingItemID_beforeInserting(t *testing.T) {
	// Given a ready store with an orphaned chunk for item-1 under an old
	// ID — the kind of stale sibling a failed Remove can leave behind (see
	// specs/phases/phase-02-knowledge-engine/08-01-vectorstore-orphan-chunk-recovery.md)
	store := New()
	ctx := context.Background()
	orphan := testChunk("old-id", []float32{1, 0})
	orphan.ItemID = "item-1"
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{orphan}))

	// When a freshly re-embedded chunk for the same item is added under a
	// new ID
	fresh := testChunk("new-id", []float32{0, 1})
	fresh.ItemID = "item-1"
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{fresh}))

	// Then the store converges to exactly the new chunk — the orphan never
	// lingers alongside it
	assert.Equal(t, 1, store.Len())
	results, err := store.Search(ctx, []float32{0, 1}, 10, knowledge.SearchFilters{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "new-id", results[0].Chunk.ID)
}

func TestStore_Add_leavesOtherItemsUntouched_whenEvictingAStaleSibling(t *testing.T) {
	// Given a ready store with chunks for two different items
	store := New()
	ctx := context.Background()
	itemOneOld := testChunk("item1-old", []float32{1, 0})
	itemOneOld.ItemID = "item-1"
	itemTwo := testChunk("item2", []float32{0, 1})
	itemTwo.ItemID = "item-2"
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{itemOneOld, itemTwo}))

	// When item-1 is re-added under a new ID
	itemOneNew := testChunk("item1-new", []float32{1, 1})
	itemOneNew.ItemID = "item-1"
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{itemOneNew}))

	// Then item-2's chunk is untouched and item-1 has only its new chunk
	assert.Equal(t, 2, store.Len())
	results, err := store.Search(ctx, []float32{0, 1}, 10, knowledge.SearchFilters{})
	require.NoError(t, err)
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.Chunk.ID
	}
	assert.ElementsMatch(t, []string{"item1-new", "item2"}, ids)
}

func TestStore_Add_preservesEveryChunkOfAMultiChunkItem_withinTheSameBatch(t *testing.T) {
	// Given an empty store — a multi-section document (e.g. a markdown file
	// with several headings) is imported as several Chunks that all share
	// one ItemID, the way internal/application/ingest/import_folder.go
	// builds its batch: one chunk per heading section, same ItemID
	store := New()
	ctx := context.Background()
	sectionOne := testChunk("doc-section-1", []float32{1, 0})
	sectionOne.ItemID = "item-doc"
	sectionTwo := testChunk("doc-section-2", []float32{0, 1})
	sectionTwo.ItemID = "item-doc"

	// When both are added together in one Add call
	require.NoError(t, store.Add(ctx, []knowledge.Chunk{sectionOne, sectionTwo}))

	// Then neither is evicted as a stale sibling of the other — the
	// by-ItemID eviction only clears chunks left behind by an *earlier*
	// call, never chunks arriving together in the same batch
	assert.Equal(t, 2, store.Len())
}

func TestStore_ReplaceAll_publishesEmptySnapshot_andMarksReady(t *testing.T) {
	// Given a fresh, never-loaded store
	store := New()
	ctx := context.Background()

	// When replacing with a genuinely empty snapshot
	err := store.ReplaceAll(ctx, nil)

	// Then it becomes ready (a real, valid, empty knowledge base) rather
	// than staying in the unavailable state
	require.NoError(t, err)
	_, searchErr := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})
	assert.NoError(t, searchErr)
	assert.Equal(t, 0, store.Len())
}

func TestStore_ReplaceAll_isAllOrNothing_precedingSnapshotUntouched(t *testing.T) {
	// Given a store with a published snapshot
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))

	// When replacing it with a batch containing one invalid chunk
	err := store.ReplaceAll(ctx, []knowledge.Chunk{
		testChunk("c2", []float32{0, 1}),
		testChunk("c3", nil),
	})

	// Then the replacement is rejected and the previous snapshot is untouched
	assert.ErrorIs(t, err, knowledge.ErrInvalidVector)
	assert.Equal(t, 1, store.Len())
	results, searchErr := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})
	require.NoError(t, searchErr)
	require.Len(t, results, 1)
	assert.Equal(t, "c1", results[0].Chunk.ID)
}

func TestStore_ReplaceAll_atomicallyReplacesThePreviousSnapshot(t *testing.T) {
	// Given a store with one chunk published
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))

	// When replacing it with a different snapshot
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c2", []float32{0, 1})}))

	// Then only the new snapshot's chunk is present
	assert.Equal(t, 1, store.Len())
	results, err := store.Search(ctx, []float32{0, 1}, 10, knowledge.SearchFilters{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "c2", results[0].Chunk.ID)
}

func TestStore_ReplaceAll_returnsContextError_evenWhenEmpty_andPreservesPreviousState(t *testing.T) {
	// Given a store with a published snapshot, and a canceled context
	store := New()
	bgCtx := context.Background()
	require.NoError(t, store.ReplaceAll(bgCtx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When replacing with a genuinely empty snapshot under the canceled context
	err := store.ReplaceAll(ctx, nil)

	// Then the context error is returned — an empty ReplaceAll is a real
	// publication, not a no-op — and the previous snapshot survives
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, store.Len())
}

func TestStore_Remove_removesEveryMatchingID(t *testing.T) {
	// Given a store with two chunks
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{
		testChunk("c1", []float32{1, 0}),
		testChunk("c2", []float32{0, 1}),
	}))

	// When removing one of them
	err := store.Remove(ctx, []string{"c1"})

	// Then only that one is gone
	require.NoError(t, err)
	assert.Equal(t, 1, store.Len())
	results, searchErr := store.Search(ctx, []float32{0, 1}, 10, knowledge.SearchFilters{})
	require.NoError(t, searchErr)
	require.Len(t, results, 1)
	assert.Equal(t, "c2", results[0].Chunk.ID)
}

func TestStore_Remove_isIdempotent_forUnknownIDs(t *testing.T) {
	// Given a store with one chunk
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))

	// When removing an ID that was never present
	err := store.Remove(ctx, []string{"never-existed"})

	// Then it succeeds as a no-op
	require.NoError(t, err)
	assert.Equal(t, 1, store.Len())
}

func TestStore_Remove_isSuccessfulNoOp_whenEmpty_evenWithCanceledContext(t *testing.T) {
	// Given a canceled context
	store := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When removing an empty ID list
	err := store.Remove(ctx, nil)

	// Then it still succeeds
	assert.NoError(t, err)
}

func TestStore_Remove_returnsErrInvalidChunkID_beforeMutatingAnything(t *testing.T) {
	// Given a store with one chunk
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))

	// When removing a valid ID alongside a blank one
	err := store.Remove(ctx, []string{"c1", "  "})

	// Then the whole call is rejected and nothing — not even the valid ID — is removed
	assert.ErrorIs(t, err, knowledge.ErrInvalidChunkID)
	assert.Equal(t, 1, store.Len())
}

func TestStore_Search_returnsErrInvalidTopK_whenTopKIsZeroOrNegative(t *testing.T) {
	// Given a ready store
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, nil))

	// When searching with topK <= 0
	_, errZero := store.Search(ctx, []float32{1, 0}, 0, knowledge.SearchFilters{})
	_, errNegative := store.Search(ctx, []float32{1, 0}, -1, knowledge.SearchFilters{})

	// Then both are rejected
	assert.ErrorIs(t, errZero, knowledge.ErrInvalidTopK)
	assert.ErrorIs(t, errNegative, knowledge.ErrInvalidTopK)
}

func TestStore_Search_returnsErrInvalidVector_forInvalidQuery(t *testing.T) {
	// Given a ready store
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, nil))

	// When searching with a zero-norm query
	_, err := store.Search(ctx, []float32{0, 0}, 1, knowledge.SearchFilters{})

	// Then it is rejected
	assert.ErrorIs(t, err, knowledge.ErrInvalidVector)
}

func TestStore_Search_filtersByTopicSourceStatus_exactMatch(t *testing.T) {
	// Given chunks spanning different topics, sources and statuses
	store := New()
	ctx := context.Background()
	goApproved := testChunk("go-approved", []float32{1, 0})
	goDraft := testChunk("go-draft", []float32{1, 0})
	goDraft.Status = knowledge.StatusDraft
	rustApproved := testChunk("rust-approved", []float32{1, 0})
	rustApproved.Topic = "Rust"
	athenaChunk := testChunk("go-athena", []float32{1, 0})
	athenaChunk.Source = knowledge.SourceAthena
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{goApproved, goDraft, rustApproved, athenaChunk}))

	// When searching with every filter set
	results, err := store.Search(ctx, []float32{1, 0}, 10, knowledge.SearchFilters{
		Topic: "Go", Source: knowledge.SourceImportedDoc, Status: knowledge.StatusApproved,
	})

	// Then only the exact match survives
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "go-approved", results[0].Chunk.ID)
}

func TestStore_Search_emptyFilters_imposeNoConstraint(t *testing.T) {
	// Given chunks with different topics
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{
		testChunk("c1", []float32{1, 0}),
		func() knowledge.Chunk { c := testChunk("c2", []float32{1, 0}); c.Topic = "Rust"; return c }(),
	}))

	// When searching with an all-empty filter
	results, err := store.Search(ctx, []float32{1, 0}, 10, knowledge.SearchFilters{})

	// Then both chunks are returned
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestStore_Search_skipsDimensionMismatchedChunks(t *testing.T) {
	// Given one chunk matching the query's dimension and one that doesn't
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{
		testChunk("matching", []float32{1, 0}),
		testChunk("mismatched", []float32{1, 0, 0}),
	}))

	// When searching with a 2-dimensional query
	results, err := store.Search(ctx, []float32{1, 0}, 10, knowledge.SearchFilters{})

	// Then only the matching-dimension chunk is scored
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "matching", results[0].Chunk.ID)
}

func TestStore_Search_ordersByScoreDescending_thenIDAscending_andTruncatesToTopK(t *testing.T) {
	// Given three chunks: two that tie in score, one that scores lower
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{
		testChunk("b-tied", []float32{1, 0}),
		testChunk("a-tied", []float32{1, 0}),
		testChunk("low", []float32{0, 1}),
	}))

	// When searching with topK smaller than the total match count
	results, err := store.Search(ctx, []float32{1, 0}, 2, knowledge.SearchFilters{})

	// Then the highest-scoring, ID-ascending-tiebroken results come first,
	// truncated to topK
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "a-tied", results[0].Chunk.ID)
	assert.Equal(t, "b-tied", results[1].Chunk.ID)
}

func TestStore_Search_scoresMatchCosineSimilarity_onRawVectors(t *testing.T) {
	// Given a chunk with a known, non-unit raw embedding
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{3, 4})}))
	query := []float32{4, 3}

	// When searching with a query that is also non-unit
	results, err := store.Search(ctx, query, 1, knowledge.SearchFilters{})

	// Then the score equals cosine similarity computed independently over
	// the original raw (un-normalized) vectors
	require.NoError(t, err)
	require.Len(t, results, 1)
	expected := independentCosine([]float32{3, 4}, []float32{4, 3})
	assert.InDelta(t, expected, results[0].Score, 1e-5)
}

func independentCosine(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func TestStore_Search_normalizesQuery_withoutMutatingCallersSlice(t *testing.T) {
	// Given a ready store and a non-unit query vector
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))
	callerQuery := []float32{3, 4}

	// When searching with it
	_, err := store.Search(ctx, callerQuery, 1, knowledge.SearchFilters{})

	// Then the caller's own query slice is untouched
	require.NoError(t, err)
	assert.Equal(t, []float32{3, 4}, callerQuery)
}

func TestStore_Search_returnsDeepCopy_thatDoesNotAliasStoredEmbedding(t *testing.T) {
	// Given a stored chunk
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))

	// When searching and then mutating the returned embedding
	results, err := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	results[0].Chunk.Embedding[0] = 999

	// Then a second search is unaffected — the caller cannot mutate the store
	again, err := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.NotEqual(t, float32(999), again[0].Chunk.Embedding[0])
}

func TestStore_Search_returnsContextError_whenCanceled(t *testing.T) {
	// Given a ready store and an already-canceled context
	store := New()
	bgCtx := context.Background()
	require.NoError(t, store.ReplaceAll(bgCtx, []knowledge.Chunk{testChunk("c1", []float32{1, 0})}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When searching
	results, err := store.Search(ctx, []float32{1, 0}, 1, knowledge.SearchFilters{})

	// Then the context error is returned, with no partial result
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, results)
}

func TestStore_ConcurrentAddSearchRemoveReplaceAllLen_doNotRace(t *testing.T) {
	// Given a store under concurrent load from every method
	store := New()
	ctx := context.Background()
	require.NoError(t, store.ReplaceAll(ctx, nil))

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(5)
		go func(i int) {
			defer wg.Done()
			_ = store.Add(ctx, []knowledge.Chunk{testChunk(fmt.Sprintf("c%d", i), []float32{1, 0})})
		}(i)
		go func() {
			defer wg.Done()
			_, _ = store.Search(ctx, []float32{1, 0}, 5, knowledge.SearchFilters{})
		}()
		go func(i int) {
			defer wg.Done()
			_ = store.Remove(ctx, []string{fmt.Sprintf("c%d", i)})
		}(i)
		go func() {
			defer wg.Done()
			_ = store.Len()
		}()
		go func(i int) {
			defer wg.Done()
			_ = store.ReplaceAll(ctx, []knowledge.Chunk{testChunk(fmt.Sprintf("r%d", i), []float32{0, 1})})
		}(i)
	}
	wg.Wait()

	// Then no assertion beyond "go test -race reports nothing" is needed
}

func TestStore_Search_10000ChunksOf1536Dimensions_returnsCorrectTopKInOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping functional 10k benchmark-scale test in -short mode")
	}
	// Given 10,000 chunks of 1536 dimensions, one of which is an exact
	// match for the query direction
	store := New()
	ctx := context.Background()
	const count = 10000
	const dims = 1536
	chunks := make([]knowledge.Chunk, count)
	for i := range count {
		vec := make([]float32, dims)
		vec[i%dims] = 1 // a distinct, low-similarity direction per chunk
		chunks[i] = testChunk(fmt.Sprintf("chunk-%05d", i), vec)
	}
	query := make([]float32, dims)
	query[0] = 1
	require.NoError(t, store.ReplaceAll(ctx, chunks))
	require.Equal(t, count, store.Len())

	// When searching for the exact-match direction
	results, err := store.Search(ctx, query, 5, knowledge.SearchFilters{})

	// Then the exact match (every chunk whose distinguishing 1 landed at
	// index 0) scores highest and results are ordered/truncated correctly
	require.NoError(t, err)
	require.Len(t, results, 5)
	assert.InDelta(t, 1.0, results[0].Score, 1e-5)
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score)
	}
}

func BenchmarkSearch10Kx1536(b *testing.B) {
	store := New()
	ctx := context.Background()
	const count = 10000
	const dims = 1536
	chunks := make([]knowledge.Chunk, count)
	for i := range count {
		vec := make([]float32, dims)
		for d := range vec {
			vec[d] = float32((i+d)%97) + 1
		}
		chunks[i] = testChunk(fmt.Sprintf("chunk-%05d", i), vec)
	}
	if err := store.ReplaceAll(ctx, chunks); err != nil {
		b.Fatal(err)
	}
	query := make([]float32, dims)
	for d := range query {
		query[d] = float32(d%13) + 1
	}

	b.ResetTimer()
	for range b.N {
		if _, err := store.Search(ctx, query, 10, knowledge.SearchFilters{}); err != nil {
			b.Fatal(err)
		}
	}
}
