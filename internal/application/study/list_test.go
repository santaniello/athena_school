package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestListSessionsByFolder_returnsEverySessionInThatFolder(t *testing.T) {
	// Given a repository with two sessions in folder-a
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	want := []domainstudy.Session{
		{ID: "s-1", Topic: "Cache invalidation", FolderID: "folder-a"},
		{ID: "s-2", Topic: "Concurrency patterns", FolderID: "folder-a"},
	}
	sessions.EXPECT().ListByFolder(context.Background(), "folder-a").Return(want, nil).Once()
	service := NewService(sessions, messages, llm, profiles, folders)

	// When listing sessions in folder-a
	got, err := service.ListSessionsByFolder(context.Background(), "folder-a")

	// Then every session in that folder is returned
	require.NoError(t, err)
	require.Equal(t, want, got)
}
