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
	// Given a catalog whose ListModels call is slow and only expected once
	catalog := mocks.NewMockModelCatalog(t)
	release := make(chan struct{})
	var calls int32
	catalog.EXPECT().ListModels(context.Background()).
		RunAndReturn(func(_ context.Context) ([]domainllm.ModelInfo, error) {
			atomic.AddInt32(&calls, 1)
			<-release
			return []domainllm.ModelInfo{{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000}}, nil
		}).
		Once()
	service := NewService(catalog)

	// When many callers concurrently request a refresh before the first
	// completes. release is held closed until every goroutine has at least
	// reached the call to RefreshContextLength, so the loader's ListModels
	// call (blocked on <-release) cannot return — and free up a slot for a
	// stray "second load" — before all of them have had a chance to join
	// the same in-flight call.
	const concurrentCallers = 10
	var wg sync.WaitGroup
	var launched int32
	wg.Add(concurrentCallers)
	for i := 0; i < concurrentCallers; i++ {
		go func() {
			defer wg.Done()
			atomic.AddInt32(&launched, 1)
			length, err := service.RefreshContextLength(context.Background(), "anthropic/claude-sonnet-4.5")
			assert.NoError(t, err)
			assert.Equal(t, 200000, length)
		}()
	}
	for atomic.LoadInt32(&launched) < concurrentCallers {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()

	// Then the catalog was only actually queried once (Mockery's .Once()
	// above already fails the test on a second call; this is an explicit
	// belt-and-suspenders check)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
