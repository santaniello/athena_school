package study

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStudySession_IsOpen_returnsTrue_whenEndedAtIsZero(t *testing.T) {
	// Given a session that has not been ended
	s := Session{StartedAt: time.Now()}

	// When checking whether it is open
	open := s.IsOpen()

	// Then it reports open
	assert.True(t, open)
}

func TestStudySession_IsOpen_returnsFalse_whenEndedAtIsSet(t *testing.T) {
	// Given a session that has been ended
	s := Session{StartedAt: time.Now(), EndedAt: time.Now()}

	// When checking whether it is open
	open := s.IsOpen()

	// Then it reports closed
	assert.False(t, open)
}
