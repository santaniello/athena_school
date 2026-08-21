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

func TestResume_returnsSessionAndFullHistory(t *testing.T) {
	// Given a service with a session that has two prior messages
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	history := []domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Hi"}}
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When resuming the session
	got, msgs, err := service.Resume(context.Background(), "session-1")

	// Then the session and its full history are returned
	require.NoError(t, err)
	require.Equal(t, session, got)
	require.Equal(t, history, msgs)
}

func TestResume_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "missing").Return(domainstudy.Session{}, domainstudy.ErrSessionNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When resuming a session that does not exist
	_, _, err := service.Resume(context.Background(), "missing")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}
