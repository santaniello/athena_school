package sessionlock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLocks() *Locks { return &Locks{} }

func TestLocks_forgetsASessionOnceNobodyHoldsOrWaitsForIt(t *testing.T) {
	// Given a lock held by one caller
	locks := newTestLocks()
	unlock := must(locks.Lock(context.Background(), "session-1"))
	assert.Equal(t, 1, locks.held["session-1"].refs)

	// When it is released
	unlock()

	// Then the session leaves the map
	assert.Empty(t, locks.held)
}

func TestLocks_keepsASessionWhileAnotherCallerStillWaitsForIt(t *testing.T) {
	// Given a session locked once, with a second caller already counted on it
	locks := newTestLocks()
	unlock := must(locks.Lock(context.Background(), "session-1"))
	locks.held["session-1"].refs++ // a waiter that took a place in line

	// When the holder releases it
	unlock()

	// Then the entry stays for the waiter, with one reference left
	assert.Equal(t, 1, locks.held["session-1"].refs)
}

func TestLocks_sessionsAreIndependent(t *testing.T) {
	// Given two sessions locked at once
	locks := newTestLocks()
	unlockOne := must(locks.Lock(context.Background(), "session-1"))
	unlockTwo := must(locks.Lock(context.Background(), "session-2"))

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

func TestLocks_aWaiterGivesUpWhenItsContextIsCancelled(t *testing.T) {
	// Given a session held by one caller and a second one waiting for it
	locks := newTestLocks()
	unlock := must(locks.Lock(context.Background(), "session-1"))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := locks.Lock(ctx, "session-1")
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

func TestLocks_aCancelledContextNeverTakesAFreeLock(t *testing.T) {
	// Given a free session and a context that is already cancelled
	locks := newTestLocks()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When it asks for the lock many times, since a ready slot and a done context used to be picked at random
	for range 200 {
		unlock, err := locks.Lock(ctx, "session-1")

		// Then it always gets the context's error and holds nothing
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, unlock)
		assert.Empty(t, locks.held)
	}
}
