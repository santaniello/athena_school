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

func TestDeleteSession_deletesSessionThroughItsRepositoryOwnership(t *testing.T) {
	// Given a service whose session repository owns dependent-row cleanup
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	sessions.EXPECT().
		Delete(context.Background(), "session-1").
		Return(nil).
		Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When deleting the session
	err := service.DeleteSession(context.Background(), "session-1")

	// Then one repository operation owns the atomic session deletion; the
	// message port receives no separate, partially committable delete
	require.NoError(t, err)
}

func TestDeleteSession_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().Delete(context.Background(), "missing").Return(domainstudy.ErrSessionNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When deleting a session that does not exist
	err := service.DeleteSession(context.Background(), "missing")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}
