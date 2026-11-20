package desktop

import (
	"context"
	"sync"
)

// generationSlot lets one generation run at a time and be cancelled.
type generationSlot struct {
	mu       sync.Mutex
	cancelFn context.CancelFunc
}

// begin reserves the slot. It returns the context the generation runs under and
// the function that frees the slot, or false when a generation already runs.
func (s *generationSlot) begin(parent context.Context) (context.Context, func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancelFn = cancel
	return ctx, func() {
		cancel()
		s.mu.Lock()
		s.cancelFn = nil
		s.mu.Unlock()
	}, true
}

// cancel stops the running generation, if any.
func (s *generationSlot) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
}
