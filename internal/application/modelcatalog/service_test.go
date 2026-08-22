package modelcatalog

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	"github.com/santaniello/athena/internal/domain/llm/mocks"
)

func TestService_CachedContextLength_missesBeforeAnyLoad(t *testing.T) {
	// Given a freshly constructed Service with no load yet
	catalog := mocks.NewMockModelCatalog(t)
	service := NewService(catalog)

	// When looking up a model without ever calling Warm/RefreshContextLength
	_, ok := service.CachedContextLength("anthropic/claude-sonnet-4.5")

	// Then it misses without touching the catalog port (no .EXPECT() set)
	assert.False(t, ok)
}

func TestService_RefreshContextLength_populatesCacheOnSuccess(t *testing.T) {
	// Given a catalog with one valid model
	catalog := mocks.NewMockModelCatalog(t)
	catalog.EXPECT().ListModels(context.Background()).
		Return([]domainllm.ModelInfo{{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000}}, nil).
		Once()
	service := NewService(catalog)

	// When refreshing for that model
	length, err := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")

	// Then it returns the context length and the cache is now populated
	require.NoError(t, err)
	assert.Equal(t, 200000, length)
	cached, ok := service.CachedContextLength("anthropic/claude-sonnet-4.5")
	assert.True(t, ok)
	assert.Equal(t, 200000, cached)
}

func TestService_RefreshContextLength_returnsError_whenModelStillMissingAfterRefresh(t *testing.T) {
	// Given a catalog that never mentions the requested model
	catalog := mocks.NewMockModelCatalog(t)
	catalog.EXPECT().ListModels(context.Background()).
		Return([]domainllm.ModelInfo{{ID: "other/model", ContextLength: 1000}}, nil).
		Once()
	service := NewService(catalog)

	// When refreshing for a model absent from the catalog
	_, err := service.RefreshContextLength(context.Background(), "missing/model")

	// Then it fails
	require.Error(t, err)
}

func TestService_load_preservesPreviousCache_onTransportFailure(t *testing.T) {
	// Given a Service with an already-populated cache
	catalog := mocks.NewMockModelCatalog(t)
	catalog.EXPECT().ListModels(context.Background()).
		Return([]domainllm.ModelInfo{{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000}}, nil).
		Once()
	service := NewService(catalog)
	_, err := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
	require.NoError(t, err)

	// When a later refresh fails at the transport level
	catalog.EXPECT().ListModels(context.Background()).Return(nil, errors.New("network down")).Once()
	_, refreshErr := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
	require.Error(t, refreshErr)

	// Then the previous cache is preserved, not replaced by an empty one
	cached, ok := service.CachedContextLength("anthropic/claude-sonnet-4.5")
	assert.True(t, ok)
	assert.Equal(t, 200000, cached)
}

func TestService_load_preservesPreviousCache_whenNewResponseHasNoValidEntries(t *testing.T) {
	// Given a Service with an already-populated cache
	catalog := mocks.NewMockModelCatalog(t)
	catalog.EXPECT().ListModels(context.Background()).
		Return([]domainllm.ModelInfo{{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000}}, nil).
		Once()
	service := NewService(catalog)
	_, err := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
	require.NoError(t, err)

	// When a later refresh succeeds at the transport level but every entry
	// is invalid (blank ID)
	catalog.EXPECT().ListModels(context.Background()).
		Return([]domainllm.ModelInfo{{ID: "", ContextLength: 1000}}, nil).
		Once()
	_, refreshErr := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
	require.Error(t, refreshErr)

	// Then the previous cache is preserved
	cached, ok := service.CachedContextLength("anthropic/claude-sonnet-4.5")
	assert.True(t, ok)
	assert.Equal(t, 200000, cached)
}

func TestBuildValidCache_entryIsolatedValidation(t *testing.T) {
	cases := []struct {
		name   string
		models []domainllm.ModelInfo
		want   map[string]int
	}{
		{
			name: "skips blank ID",
			models: []domainllm.ModelInfo{
				{ID: "", ContextLength: 1000},
				{ID: "valid/model", ContextLength: 2000},
			},
			want: map[string]int{"valid/model": 2000},
		},
		{
			name: "skips non-positive context length",
			models: []domainllm.ModelInfo{
				{ID: "zero/length", ContextLength: 0},
				{ID: "negative/length", ContextLength: -5},
				{ID: "valid/model", ContextLength: 2000},
			},
			want: map[string]int{"valid/model": 2000},
		},
		{
			name: "collapses identical duplicates",
			models: []domainllm.ModelInfo{
				{ID: "dup/model", ContextLength: 4000},
				{ID: "dup/model", ContextLength: 4000},
			},
			want: map[string]int{"dup/model": 4000},
		},
		{
			name: "excludes conflicting duplicates entirely",
			models: []domainllm.ModelInfo{
				{ID: "conflict/model", ContextLength: 4000},
				{ID: "conflict/model", ContextLength: 8000},
				{ID: "valid/model", ContextLength: 2000},
			},
			want: map[string]int{"valid/model": 2000},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := buildValidCache(c.models)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestBuildValidCache_returnsError_whenNoValidEntriesRemain(t *testing.T) {
	// Given a response whose every entry is invalid
	_, err := buildValidCache([]domainllm.ModelInfo{{ID: "", ContextLength: 0}})

	// Then it fails rather than caching an empty map
	require.ErrorIs(t, err, ErrEmptyCatalog)
}

func TestService_RefreshContextLength_singleFlight_coalescesConcurrentCallers(t *testing.T) {
	// Given a catalog whose ListModels call is slow. Deliberately no
	// .Once()/.Times() constraint here: if the timing barriers below still
	// let a straggler start a second load (see the comment on them), a
	// mockery call-count violation would call t.FailNow() from inside that
	// goroutine's call to ListModels — which unwinds via runtime.Goexit()
	// straight past the direct (non-deferred) close(call.done) in
	// refresh(), permanently blocking every other waiter on that call's
	// done channel. An occasional extra call is an acceptable, visible
	// assertion failure below; a goroutine leak that hangs the whole test
	// binary for its full timeout is not.
	catalog := mocks.NewMockModelCatalog(t)
	release := make(chan struct{})
	var calls int32
	catalog.EXPECT().ListModels(context.Background()).
		RunAndReturn(func(_ context.Context) ([]domainllm.ModelInfo, error) {
			atomic.AddInt32(&calls, 1)
			<-release
			return []domainllm.ModelInfo{{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000}}, nil
		})
	service := NewService(catalog)

	// When many callers concurrently request a refresh before the first
	// completes. Two barriers make that overlap the overwhelmingly likely
	// outcome, though neither is a hard guarantee:
	//  1. The first caller is launched alone and awaited until it is
	//     observably inside ListModels (calls == 1) — by that point
	//     s.flight is already set, since refresh() assigns it before
	//     calling load(). Only then are the other 9 launched, so none of
	//     them can start before s.flight is non-nil.
	//  2. release is only closed once all 9 have incremented launched,
	//     which they do immediately before calling RefreshContextLength —
	//     s.flight cannot become nil before release closes (the loader is
	//     still blocked on <-release).
	// What neither barrier rules out: a goroutine preempted between
	// incrementing launched and actually acquiring the lock inside
	// refresh() could in principle resume after the loader has already
	// finished and cleared s.flight, and start a second load. No blocking
	// call sits in that gap, so it is not realistically reachable — but,
	// per the comment above, the test tolerates it via a failed assertion
	// rather than a hang if it ever happens.
	const concurrentCallers = 10
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		length, err := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
		assert.NoError(t, err)
		assert.Equal(t, 200000, length)
	}()
	for atomic.LoadInt32(&calls) < 1 {
		runtime.Gosched()
	}

	var launched int32
	wg.Add(concurrentCallers - 1)
	for i := 0; i < concurrentCallers-1; i++ {
		go func() {
			defer wg.Done()
			atomic.AddInt32(&launched, 1)
			length, err := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
			assert.NoError(t, err)
			assert.Equal(t, 200000, length)
		}()
	}
	for atomic.LoadInt32(&launched) < concurrentCallers-1 {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()

	// Then the catalog was queried only once
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
