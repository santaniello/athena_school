package study

import "sync"

// inFlightCoordinator is a process-local, non-blocking, per-session
// reservation: at most one opening/study generation may run for a given
// session at a time; different sessions remain independent. Modeled on
// internal/application/knowledge.IndexLoader's opMu TryLock pattern, keyed
// by session ID instead of a single global lock.
type inFlightCoordinator struct {
	mu       sync.Mutex
	sessions map[string]struct{}
}

func newInFlightCoordinator() *inFlightCoordinator {
	return &inFlightCoordinator{sessions: make(map[string]struct{})}
}

// begin reserves sessionID, or returns ErrStudyTurnInProgress immediately
// if it is already reserved. Callers must call end exactly once after a
// nil return, typically via defer.
func (c *inFlightCoordinator) begin(sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.sessions[sessionID]; ok {
		return ErrStudyTurnInProgress
	}
	c.sessions[sessionID] = struct{}{}
	return nil
}

// end releases the reservation acquired by a successful begin.
func (c *inFlightCoordinator) end(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, sessionID)
}
