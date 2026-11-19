// Package sessionlock serializes work per study session, in process: the
// desktop app is a single process. It depends on the standard library only, so
// every use case that needs it can import it without knowing the others.
package sessionlock

import (
	"context"
	"sync"
)

// Locks hands out one lock per session. A caller waiting for a lock gives up
// when its context is done, so a cancelled request does not sit behind a long
// generation. An entry lives only while someone holds or waits for it, so the
// map does not grow with the sessions ever seen. The zero value is ready to
// use; a Locks must not be copied after first use.
type Locks struct {
	mu   sync.Mutex
	held map[string]*entry
}

// entry is a one-slot semaphore: a channel, unlike a mutex, can be waited on
// together with a context.
type entry struct {
	slot chan struct{}
	refs int
}

// Lock takes the session's lock and returns the function that releases it. It
// returns the context's error, holding nothing, when ctx is done first.
func (l *Locks) Lock(ctx context.Context, sessionID string) (func(), error) {
	l.mu.Lock()
	if l.held == nil {
		l.held = map[string]*entry{}
	}
	e, ok := l.held[sessionID]
	if !ok {
		e = &entry{slot: make(chan struct{}, 1)}
		l.held[sessionID] = e
	}
	e.refs++
	l.mu.Unlock()
	release := func() {
		l.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(l.held, sessionID)
		}
		l.mu.Unlock()
	}
	select {
	case e.slot <- struct{}{}:
		return func() {
			<-e.slot
			release()
		}, nil
	case <-ctx.Done():
		release()
		return nil, ctx.Err()
	}
}
