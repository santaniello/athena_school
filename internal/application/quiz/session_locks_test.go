package quiz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestLocks() *sessionLocks { return &sessionLocks{held: map[string]*sessionLock{}} }

func TestSessionLocks_forgetsASessionOnceNobodyHoldsOrWaitsForIt(t *testing.T) {
	// Given a lock held by one caller
	locks := newTestLocks()
	unlock := locks.lock("session-1")
	assert.Equal(t, 1, locks.held["session-1"].refs)

	// When it is released
	unlock()

	// Then the session leaves the map
	assert.Empty(t, locks.held)
}

func TestSessionLocks_keepsASessionWhileAnotherCallerStillWaitsForIt(t *testing.T) {
	// Given a session locked once, with a second caller already counted on it
	locks := newTestLocks()
	unlock := locks.lock("session-1")
	locks.held["session-1"].refs++ // a waiter that took a place in line

	// When the holder releases it
	unlock()

	// Then the entry stays for the waiter, with one reference left
	assert.Equal(t, 1, locks.held["session-1"].refs)
}

func TestSessionLocks_sessionsAreIndependent(t *testing.T) {
	// Given two sessions locked at once
	locks := newTestLocks()
	unlockOne := locks.lock("session-1")
	unlockTwo := locks.lock("session-2")

	// Then each has its own entry, and releasing one keeps the other
	assert.Len(t, locks.held, 2)
	unlockOne()
	assert.Len(t, locks.held, 1)
	unlockTwo()
	assert.Empty(t, locks.held)
}
