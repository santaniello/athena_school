package quiz

import "sync"

// sessionLocks hands out one mutex per session, in process: the desktop app is
// a single process. An entry lives only while someone holds or waits for it, so
// the map does not grow with the sessions ever seen. It has the shape of the
// exercise offers' lock on purpose: the two contexts do not share code.
type sessionLocks struct {
	mu   sync.Mutex
	held map[string]*sessionLock
}

type sessionLock struct {
	mu   sync.Mutex
	refs int
}

// lock takes the session's mutex and returns the function that releases it.
func (l *sessionLocks) lock(sessionID string) func() {
	l.mu.Lock()
	entry, ok := l.held[sessionID]
	if !ok {
		entry = &sessionLock{}
		l.held[sessionID] = entry
	}
	entry.refs++
	l.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.held, sessionID)
		}
		l.mu.Unlock()
	}
}
