package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestMoveToFolder_movesTheSession(t *testing.T) {
	// Given a service whose repository accepts the move
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().MoveToFolder(context.Background(), "session-1", "folder-b").Return(nil).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When moving the session to another folder
	err := service.MoveToFolder(context.Background(), "session-1", "folder-b")

	// Then it succeeds
	require.NoError(t, err)
}

func TestMoveToFolder_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().MoveToFolder(context.Background(), "missing", "folder-b").Return(domainstudy.ErrSessionNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When moving a session that does not exist
	err := service.MoveToFolder(context.Background(), "missing", "folder-b")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}
