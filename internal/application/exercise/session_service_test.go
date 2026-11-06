package exercise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

func TestSessionService_ListSessionsForGeneration_returnsTheRepositorySessionsInOrder(t *testing.T) {
	// Given two sessions stored across folders, newest first
	repo := domainmocks.NewMockSessionOptionRepository(t)
	want := []domainexercise.SessionOption{
		{SessionID: "s2", Topic: "Caching", FolderName: "Backend", StartedAt: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)},
		{SessionID: "s1", Topic: "Queues", FolderName: "Infra", StartedAt: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)},
	}
	repo.EXPECT().ListAll(context.Background()).Return(want, nil)
	service := NewSessionService(repo)

	// When listing the sessions for generation
	got, err := service.ListSessionsForGeneration(context.Background())

	// Then they come back unchanged
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSessionService_ListSessionsForGeneration_returnsAnEmptyListWhenThereAreNoSessions(t *testing.T) {
	// Given a repository with no sessions
	repo := domainmocks.NewMockSessionOptionRepository(t)
	repo.EXPECT().ListAll(context.Background()).Return(nil, nil)
	service := NewSessionService(repo)

	// When listing the sessions for generation
	got, err := service.ListSessionsForGeneration(context.Background())

	// Then the list is empty, never nil, so the frontend receives []
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSessionService_ListSessionsForGeneration_wrapsRepositoryErrors(t *testing.T) {
	// Given a repository that fails
	boom := errors.New("boom")
	repo := domainmocks.NewMockSessionOptionRepository(t)
	repo.EXPECT().ListAll(context.Background()).Return(nil, boom)
	service := NewSessionService(repo)

	// When listing the sessions for generation
	_, err := service.ListSessionsForGeneration(context.Background())

	// Then the error is wrapped
	require.ErrorIs(t, err, boom)
}
