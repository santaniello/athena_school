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

func TestDeleteSession_deletesMessagesBeforeTheSession(t *testing.T) {
	// Given a service tracking the order ports are called in
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	var callOrder []string
	messages.EXPECT().
		DeleteBySession(context.Background(), "session-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete-messages") }).
		Return(nil).
		Once()
	sessions.EXPECT().
		Delete(context.Background(), "session-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete-session") }).
		Return(nil).
		Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When deleting the session
	err := service.DeleteSession(context.Background(), "session-1")

	// Then its messages are deleted before the session itself
	require.NoError(t, err)
	require.Equal(t, []string{"delete-messages", "delete-session"}, callOrder)
}

func TestDeleteSession_doesNotDeleteSession_whenDeletingMessagesFails(t *testing.T) {
	// Given a service whose message deletion fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().DeleteBySession(context.Background(), "session-1").Return(domainstudy.ErrSessionNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When deleting the session
	err := service.DeleteSession(context.Background(), "session-1")

	// Then the error propagates; sessions.Delete has no .EXPECT(), so an
	// unexpected call would fail the test
	require.Error(t, err)
}

func TestDeleteSession_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().DeleteBySession(context.Background(), "missing").Return(nil).Once()
	sessions.EXPECT().Delete(context.Background(), "missing").Return(domainstudy.ErrSessionNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When deleting a session that does not exist
	err := service.DeleteSession(context.Background(), "missing")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}
