package mindmap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestLocks() *sessionLocks { return &sessionLocks{held: map[string]*sessionLock{}} }

func TestSessionLocks_forgetsASessionOnceNobodyHoldsOrWaitsForIt(t *testing.T) {
	// Given a lock held by one caller
	locks := newTestLocks()
	unlock := must(locks.lock(context.Background(), "session-1"))
	assert.Equal(t, 1, locks.held["session-1"].refs)

	// When it is released
	unlock()

	// Then the session leaves the map
	assert.Empty(t, locks.held)
}

func TestSessionLocks_keepsASessionWhileAnotherCallerStillWaitsForIt(t *testing.T) {
	// Given a session locked once, with a second caller already counted on it
	locks := newTestLocks()
	unlock := must(locks.lock(context.Background(), "session-1"))
	locks.held["session-1"].refs++ // a waiter that took a place in line

	// When the holder releases it
	unlock()

	// Then the entry stays for the waiter, with one reference left
	assert.Equal(t, 1, locks.held["session-1"].refs)
}

func TestSessionLocks_sessionsAreIndependent(t *testing.T) {
	// Given two sessions locked at once
	locks := newTestLocks()
	unlockOne := must(locks.lock(context.Background(), "session-1"))
	unlockTwo := must(locks.lock(context.Background(), "session-2"))

	// Then each has its own entry, and releasing one keeps the other
	assert.Len(t, locks.held, 2)
	unlockOne()
	assert.Len(t, locks.held, 1)
	unlockTwo()
	assert.Empty(t, locks.held)
}

func must(unlock func(), err error) func() {
	if err != nil {
		panic(err)
	}
	return unlock
}

func TestSessionLocks_aWaiterGivesUpWhenItsContextIsCancelled(t *testing.T) {
	// Given a session held by one caller and a second one waiting for it
	locks := newTestLocks()
	unlock := must(locks.lock(context.Background(), "session-1"))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := locks.lock(ctx, "session-1")
		result <- err
	}()

	// When the waiter's context is cancelled
	cancel()

	// Then it returns the context's error without waiting for the holder
	assert.ErrorIs(t, <-result, context.Canceled)
	// And its place in line is given back, so the holder's release empties the map
	assert.Equal(t, 1, locks.held["session-1"].refs)
	unlock()
	assert.Empty(t, locks.held)
}
