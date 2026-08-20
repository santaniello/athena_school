package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

const testEmbeddingModel = "openai/text-embedding-3-small"

func testLoadChunk(id string) domainknowledge.Chunk {
	return domainknowledge.Chunk{
		ID: id, Source: domainknowledge.SourceImportedDoc, Topic: "Go",
		Status: domainknowledge.StatusApproved, ItemID: "item-" + id,
		Embedding: []float32{1, 0},
	}
}

func TestNewIndexLoader_startsLoading_withNoSnapshot(t *testing.T) {
	// Given a freshly constructed loader, before LoadInitial ever runs
	loader := NewIndexLoader(knowledgemocks.NewMockChunkRepository(t), knowledgemocks.NewMockVectorStore(t), testEmbeddingModel)

	// When reading its status
	status := loader.Status()

	// Then it already reports loading with no snapshot — no window where an
	// unstarted loader looks like anything else
	assert.Equal(t, domainknowledge.IndexStateLoading, status.State)
	assert.False(t, status.HasSnapshot)
}

func TestCheckMutationAllowed_returnsErrIndexLoading_beforeTheFirstLoadCompletes(t *testing.T) {
	// Given a freshly constructed loader
	loader := NewIndexLoader(knowledgemocks.NewMockChunkRepository(t), knowledgemocks.NewMockVectorStore(t), testEmbeddingModel)

	// When checking whether a mutation may proceed
	err := loader.CheckMutationAllowed()

	// Then it is blocked
	assert.ErrorIs(t, err, ErrIndexLoading)
}

func TestLoadInitial_succeeds_withNoIssues_becomesReady(t *testing.T) {
	// Given a repository reporting two clean chunks
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	valid := []domainknowledge.Chunk{testLoadChunk("c1"), testLoadChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{Chunks: valid}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, valid).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)

	// When the initial load runs
	loader.LoadInitial(ctx)

	// Then the index becomes ready, with a snapshot and no issues
	status := loader.Status()
	assert.Equal(t, domainknowledge.IndexStateReady, status.State)
	assert.True(t, status.HasSnapshot)
	assert.Empty(t, status.Issues)
	assert.NoError(t, loader.CheckMutationAllowed())
}

func TestLoadInitial_succeeds_withIssues_becomesReadyWithWarnings(t *testing.T) {
	// Given a repository reporting one valid chunk and one repository-level issue
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	valid := []domainknowledge.Chunk{testLoadChunk("c1")}
	repoIssue := domainknowledge.ChunkLoadIssue{ChunkID: "c2", Reason: domainknowledge.ChunkIssueMissingItem}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{Chunks: valid, Issues: []domainknowledge.ChunkLoadIssue{repoIssue}}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, valid).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)

	// When the initial load runs
	loader.LoadInitial(ctx)

	// Then the index becomes ready_with_warnings, carrying the repository's issue
	status := loader.Status()
	assert.Equal(t, domainknowledge.IndexStateReadyWithWarnings, status.State)
	assert.True(t, status.HasSnapshot)
	require.Len(t, status.Issues, 1)
	assert.Equal(t, "c2", status.Issues[0].ChunkID)
}

func TestLoadInitial_isolatesAnInvalidRepositoryChunk_asDefenseInDepth(t *testing.T) {
	// Given a repository that (incorrectly) returns one structurally invalid
	// chunk among its "safe" Chunks — the loader re-validates before publishing
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	good := testLoadChunk("c1")
	bad := testLoadChunk("c2")
	bad.Embedding = nil
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{Chunks: []domainknowledge.Chunk{good, bad}}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, []domainknowledge.Chunk{good}).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)

	// When the initial load runs
	loader.LoadInitial(ctx)

	// Then only the valid chunk is published, and the invalid one is
	// reported instead of silently dropped or silently included
	status := loader.Status()
	assert.Equal(t, domainknowledge.IndexStateReadyWithWarnings, status.State)
	require.Len(t, status.Issues, 1)
	assert.Equal(t, "c2", status.Issues[0].ChunkID)
	assert.Equal(t, domainknowledge.ChunkIssueInvalidVector, status.Issues[0].Reason)
}

func TestLoadInitial_fails_onRepositoryError_becomesFailedWithNoSnapshot(t *testing.T) {
	// Given a repository that fails entirely
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	boom := errors.New("disk full")
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{}, boom).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)

	// When the initial load runs
	loader.LoadInitial(ctx)

	// Then the index is failed, with no snapshot and a safe (non-raw) error
	// summary — but knowledge mutations are NOT blocked: "Continue without
	// local search" must still let SQLite-backed import/edit/approve/
	// deprecate/delete work, each reconciling into the store best-effort
	status := loader.Status()
	assert.Equal(t, domainknowledge.IndexStateFailed, status.State)
	assert.False(t, status.HasSnapshot)
	assert.NotEmpty(t, status.LastError)
	assert.NotContains(t, status.LastError, "disk full")
	assert.NoError(t, loader.CheckMutationAllowed())
}

func TestLoadInitial_fails_onStorePublishError_becomesFailed(t *testing.T) {
	// Given a repository that succeeds but a store that rejects the snapshot
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	valid := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{Chunks: valid}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, valid).Return(errors.New("store exploded")).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)

	// When the initial load runs
	loader.LoadInitial(ctx)

	// Then the index is failed
	status := loader.Status()
	assert.Equal(t, domainknowledge.IndexStateFailed, status.State)
	assert.False(t, status.HasSnapshot)
}

func TestRetry_succeeds_replacesSnapshot_andReturnsReady(t *testing.T) {
	// Given an already-ready loader
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	first := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)

	// When retrying and the reload succeeds with a different snapshot
	second := []domainknowledge.Chunk{testLoadChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: second}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, second).Return(nil).Once()
	status := loader.Retry(ctx)

	// Then the new snapshot is published and reported ready
	assert.Equal(t, domainknowledge.IndexStateReady, status.State)
	assert.True(t, status.HasSnapshot)
	assert.Equal(t, status, loader.Status())
}

func TestRetry_fails_withPriorSnapshot_restoresThePreviousReadyState(t *testing.T) {
	// Given an already-ready loader with no issues
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	first := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)

	// When a retry's reload fails
	boom := errors.New("disk full")
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{}, boom).Once()
	status := loader.Retry(ctx)

	// Then the coordinator restores the preceding ready state — keeping the
	// old snapshot searchable — rather than reporting a global failure, but
	// still records the retry's own error
	assert.Equal(t, domainknowledge.IndexStateReady, status.State)
	assert.True(t, status.HasSnapshot)
	assert.NotEmpty(t, status.LastError)
	assert.NoError(t, loader.CheckMutationAllowed())
	assert.Equal(t, status, loader.Status())
}

func TestRetry_fails_withNoPriorSnapshot_becomesFailed(t *testing.T) {
	// Given a loader whose initial load already failed (no snapshot)
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	boom := errors.New("disk full")
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{}, boom).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)

	// When retrying and it fails again
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{}, boom).Once()
	status := loader.Retry(ctx)

	// Then it is still failed — there was never a snapshot to fall back to
	assert.Equal(t, domainknowledge.IndexStateFailed, status.State)
	assert.False(t, status.HasSnapshot)
}

func TestRetry_setsStateToLoading_whilePreservingHasSnapshot_duringTheReload(t *testing.T) {
	// Given an already-ready loader
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	first := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)

	// When a retry is in flight (blocked mid-reload)
	release := make(chan struct{})
	observedDuringReload := make(chan domainknowledge.IndexStatus, 1)
	second := []domainknowledge.Chunk{testLoadChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).RunAndReturn(
		func(context.Context, string) (domainknowledge.ChunkLoadResult, error) {
			observedDuringReload <- loader.Status()
			<-release
			return domainknowledge.ChunkLoadResult{Chunks: second}, nil
		},
	).Once()
	store.EXPECT().ReplaceAll(ctx, second).Return(nil).Once()

	done := make(chan domainknowledge.IndexStatus, 1)
	go func() { done <- loader.Retry(ctx) }()

	// Then, while blocked, status is loading but HasSnapshot is still true
	// (search keeps serving the old snapshot) and mutations are blocked
	select {
	case mid := <-observedDuringReload:
		assert.Equal(t, domainknowledge.IndexStateLoading, mid.State)
		assert.True(t, mid.HasSnapshot)
		assert.ErrorIs(t, loader.CheckMutationAllowed(), ErrIndexLoading)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the retry to reach the reload")
	}
	close(release)

	select {
	case final := <-done:
		assert.Equal(t, domainknowledge.IndexStateReady, final.State)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Retry to return")
	}
}

func TestBeginMutation_returnsErrIndexLoading_immediately_whileAReloadIsInProgress(t *testing.T) {
	// Given an already-ready loader whose Retry is blocked mid-reload
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	first := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)

	release := make(chan struct{})
	reachedReload := make(chan struct{})
	second := []domainknowledge.Chunk{testLoadChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).RunAndReturn(
		func(context.Context, string) (domainknowledge.ChunkLoadResult, error) {
			close(reachedReload)
			<-release
			return domainknowledge.ChunkLoadResult{Chunks: second}, nil
		},
	).Once()
	store.EXPECT().ReplaceAll(ctx, second).Return(nil).Once()

	done := make(chan domainknowledge.IndexStatus, 1)
	go func() { done <- loader.Retry(ctx) }()
	select {
	case <-reachedReload:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the retry to reach the reload")
	}

	// When a mutation tries to begin while the reload holds the reservation
	err := loader.BeginMutation()

	// Then it is rejected immediately, matching CheckMutationAllowed's
	// contract — not blocked until the reload finishes
	assert.ErrorIs(t, err, ErrIndexLoading)

	close(release)
	<-done
}

func TestReload_waitsForAnInFlightMutation_beforeReadingOrPublishing(t *testing.T) {
	// Given an already-ready loader, with a mutation currently holding the
	// reservation (its transaction committed, its VectorStore reconciliation
	// not run yet)
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	first := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)
	require.NoError(t, loader.BeginMutation())

	// When a retry is triggered concurrently
	reloadStarted := make(chan struct{})
	second := []domainknowledge.Chunk{testLoadChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).RunAndReturn(
		func(context.Context, string) (domainknowledge.ChunkLoadResult, error) {
			close(reloadStarted)
			return domainknowledge.ChunkLoadResult{Chunks: second}, nil
		},
	).Once()
	store.EXPECT().ReplaceAll(ctx, second).Return(nil).Once()

	done := make(chan domainknowledge.IndexStatus, 1)
	go func() { done <- loader.Retry(ctx) }()

	// Then the reload does not even start reading while the mutation still
	// holds the reservation — otherwise it could read a pre-commit snapshot
	// and publish it after the mutation's own reconciliation, resurrecting
	// stale content (see delete.go's DeleteItem)
	select {
	case <-reloadStarted:
		t.Fatal("reload started before the in-flight mutation released its reservation")
	case <-time.After(100 * time.Millisecond):
	}

	// When the mutation ends
	loader.EndMutation()

	// Then the reload proceeds and completes normally
	select {
	case status := <-done:
		assert.Equal(t, domainknowledge.IndexStateReady, status.State)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Retry to return after EndMutation")
	}
}

func TestReload_serializesTwoConcurrentRetries_soTheirPublishesCannotInterleave(t *testing.T) {
	// Given an already-ready loader
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	first := []domainknowledge.Chunk{testLoadChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)
	loader.LoadInitial(ctx)

	// When two retries race, the first one blocked mid-reload
	release := make(chan struct{})
	firstReloadStarted := make(chan struct{})
	second := []domainknowledge.Chunk{testLoadChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).RunAndReturn(
		func(context.Context, string) (domainknowledge.ChunkLoadResult, error) {
			close(firstReloadStarted)
			<-release
			return domainknowledge.ChunkLoadResult{Chunks: second}, nil
		},
	).Once()
	store.EXPECT().ReplaceAll(ctx, second).Return(nil).Once()

	firstDone := make(chan struct{})
	go func() { loader.Retry(ctx); close(firstDone) }()
	select {
	case <-firstReloadStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first retry to reach the reload")
	}

	secondReloadStarted := make(chan struct{})
	third := []domainknowledge.Chunk{testLoadChunk("c3")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).RunAndReturn(
		func(context.Context, string) (domainknowledge.ChunkLoadResult, error) {
			close(secondReloadStarted)
			return domainknowledge.ChunkLoadResult{Chunks: third}, nil
		},
	).Once()
	store.EXPECT().ReplaceAll(ctx, third).Return(nil).Once()

	secondDone := make(chan domainknowledge.IndexStatus, 1)
	go func() { secondDone <- loader.Retry(ctx) }()

	// Then the second retry's reload does not start while the first is still
	// mid-flight — otherwise their ReplaceAll calls could land out of order
	select {
	case <-secondReloadStarted:
		t.Fatal("second retry's reload started before the first one finished")
	case <-time.After(100 * time.Millisecond):
	}

	// When the first retry is released and completes
	close(release)
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first retry to return")
	}

	// Then the second retry's reload now proceeds and completes
	select {
	case status := <-secondDone:
		assert.Equal(t, domainknowledge.IndexStateReady, status.State)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the second retry to return")
	}
}

func TestStatus_isSafeForConcurrentUse(t *testing.T) {
	// Given a loader with LoadInitial already running in the background
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{}, nil).Maybe()
	store.EXPECT().ReplaceAll(ctx, []domainknowledge.Chunk{}).Return(nil).Maybe()
	loader := NewIndexLoader(chunks, store, testEmbeddingModel)

	done := make(chan struct{})
	go func() {
		loader.LoadInitial(ctx)
		close(done)
	}()

	// When Status is polled concurrently with the load
	for range 50 {
		_ = loader.Status()
	}
	<-done

	// Then the load settled into a terminal state
	assert.NotEqual(t, domainknowledge.IndexStateLoading, loader.Status().State)
}
