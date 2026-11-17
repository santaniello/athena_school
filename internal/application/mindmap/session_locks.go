package mindmap

import (
	"context"
	"sync"
)

// sessionLocks hands out one lock per session, in process: the desktop app is
// a single process. A caller waiting for a lock gives up when its context is
// done, so a cancelled request does not sit behind a long generation. An entry lives only while someone holds or waits for it, so
// the map does not grow with the sessions ever seen. It has the shape of the
// exercise offers' lock on purpose: the two contexts do not share code.
type sessionLocks struct {
	mu   sync.Mutex
	held map[string]*sessionLock
}

// sessionLock is a one-slot semaphore: a channel, unlike a mutex, can be waited
// on together with a context.
type sessionLock struct {
	slot chan struct{}
	refs int
}

// lock takes the session's lock and returns the function that releases it. It
// returns the context's error, holding nothing, when ctx is done first.
func (l *sessionLocks) lock(ctx context.Context, sessionID string) (func(), error) {
	l.mu.Lock()
	entry, ok := l.held[sessionID]
	if !ok {
		entry = &sessionLock{slot: make(chan struct{}, 1)}
		l.held[sessionID] = entry
	}
	entry.refs++
	l.mu.Unlock()
	release := func() {
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.held, sessionID)
		}
		l.mu.Unlock()
	}
	select {
	case entry.slot <- struct{}{}:
		return func() {
			<-entry.slot
			release()
		}, nil
	case <-ctx.Done():
		release()
		return nil, ctx.Err()
	}
}
