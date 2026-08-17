package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"

	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestEnd_setsEndedAtOnTheSession(t *testing.T) {
	// Given a service backed by a repository that accepts ending a session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	sessions.EXPECT().
		End(context.Background(), "session-1", mock.AnythingOfType("time.Time")).
		Return(nil).
		Once()
	service := NewService(sessions, messages, llm, profiles)

	// When ending the session
	err := service.End(context.Background(), "session-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestEnd_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	sessions.EXPECT().
		End(context.Background(), "missing", mock.AnythingOfType("time.Time")).
		Return(domainstudy.ErrSessionNotFound).
		Once()
	service := NewService(sessions, messages, llm, profiles)

	// When ending a session that does not exist
	err := service.End(context.Background(), "missing")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}
