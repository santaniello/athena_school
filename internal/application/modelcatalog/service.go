// Package modelcatalog caches a provider's model catalog in memory and
// resolves exact model IDs to their context window length. See
// specs/phases/phase-02-knowledge-engine/06-study-context-limits.md.
package modelcatalog

import (
	"context"
	"sync"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// Service implements domainllm.ModelContextResolver against a
// domainllm.ModelCatalog, keeping at most one catalog request in flight at
// a time (single-flight: concurrent callers share the same in-progress
// load's result) and never letting a failed load erase a previously valid
// cache.
type Service struct {
	catalog domainllm.ModelCatalog

	mu     sync.RWMutex
	models map[string]int // model ID -> context length

	flightMu sync.Mutex
	flight   *loadCall
}

// loadCall is the state of one in-progress refresh; concurrent callers
// join it by waiting on done instead of starting a second HTTP request.
type loadCall struct {
	done chan struct{}
	err  error
}

// NewService creates a Service with an empty cache; call Warm once at
// application startup to populate it asynchronously.
func NewService(catalog domainllm.ModelCatalog) *Service {
	return &Service{catalog: catalog, models: map[string]int{}}
}

// Warm starts (or joins) a catalog load. Intended to be called once, in a
// goroutine, at application launch; its result is discarded (a failure here
// is silent — see the spec's "Catalog failure never blocks chat").
func (s *Service) Warm(ctx context.Context) {
	_ = s.refresh(ctx)
}

// CachedContextLength is a memory-only lookup and never performs I/O, so
// the successful-response path can call it without risking a network wait.
func (s *Service) CachedContextLength(modelID string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	length, ok := s.models[modelID]
	return length, ok
}

// RefreshContextLength performs or joins a catalog refresh and then repeats
// the exact-ID lookup. Concurrent calls for different (or the same) model
// IDs share a single in-flight HTTP request.
func (s *Service) RefreshContextLength(ctx context.Context, modelID string) (int, error) {
	if err := s.refresh(ctx); err != nil {
		return 0, err
	}
	length, ok := s.CachedContextLength(modelID)
	if !ok {
		return 0, ErrEmptyCatalog
	}
	return length, nil
}

// refresh performs the single-flight catalog load: the first caller starts
// it, every concurrent caller waits on the same loadCall.
func (s *Service) refresh(ctx context.Context) error {
	s.flightMu.Lock()
	if s.flight != nil {
		call := s.flight
		s.flightMu.Unlock()
		select {
		case <-call.done:
			return call.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	call := &loadCall{done: make(chan struct{})}
	s.flight = call
	s.flightMu.Unlock()

	err := s.load(ctx)

	// Clearing s.flight and closing call.done must happen atomically under
	// flightMu: if a late caller could observe s.flight == nil before
	// call.done closes, it would start a second load while this call's own
	// waiters are still parked on <-call.done, breaking the "at most one
	// request in flight, concurrent callers share its result" guarantee —
	// and if that second load fails, this call's waiters would never
	// unblock at all, since call.done is only ever closed here.
	s.flightMu.Lock()
	call.err = err
	s.flight = nil
	close(call.done)
	s.flightMu.Unlock()
	return err
}

// load fetches the catalog and, on a valid response, atomically replaces
// the cache. A transport failure or an all-invalid response preserves the
// previous cache unchanged.
func (s *Service) load(ctx context.Context) error {
	models, err := s.catalog.ListModels(ctx)
	if err != nil {
		return err
	}

	cache, err := buildValidCache(models)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.models = cache
	s.mu.Unlock()
	return nil
}

// buildValidCache applies entry-isolated validation: a blank ID or
// non-positive context length skips that entry; identical duplicate IDs
// collapse; duplicate IDs with conflicting lengths exclude that ID
// entirely. A result with no valid entries is a failed load.
func buildValidCache(models []domainllm.ModelInfo) (map[string]int, error) {
	result := map[string]int{}
	conflicted := map[string]bool{}
	for _, m := range models {
		if m.ID == "" || m.ContextLength <= 0 {
			continue
		}
		if existing, ok := result[m.ID]; ok {
			if existing != m.ContextLength {
				conflicted[m.ID] = true
			}
			continue
		}
		result[m.ID] = m.ContextLength
	}
	for id := range conflicted {
		delete(result, id)
	}
	if len(result) == 0 {
		return nil, ErrEmptyCatalog
	}
	return result, nil
}
