package study

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	studymocks "github.com/santaniello/athena/internal/application/study/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// runDeleteThroughCascade makes the mocked KnowledgeCascade behave like the
// real one: it invokes the delete callback it was handed and returns its result.
func runDeleteThroughCascade(cascade *studymocks.MockKnowledgeCascade, sessionIDs []string) {
	cascade.EXPECT().
		DeleteSessionsWithKnowledge(context.Background(), sessionIDs, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ []string, deleteSessions func(context.Context) error) error {
			return deleteSessions(ctx)
		}).
		Once()
}

func newDeleteTestService(
	t *testing.T, sessions *domainstudymocks.MockSessionRepository, cascade *studymocks.MockKnowledgeCascade,
) *Service {
	return NewService(
		sessions, domainstudymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t),
		nil, nil, nil, cascade,
	)
}

func TestDeleteSession_deletesTheSessionThroughTheKnowledgeCascade(t *testing.T) {
	// Given a service whose knowledge cascade wraps the session repository delete
	sessions := domainstudymocks.NewMockSessionRepository(t)
	cascade := studymocks.NewMockKnowledgeCascade(t)
	sessions.EXPECT().Delete(context.Background(), "session-1").Return(nil).Once()
	runDeleteThroughCascade(cascade, []string{"session-1"})
	service := newDeleteTestService(t, sessions, cascade)

	// When deleting the session
	err := service.DeleteSession(context.Background(), "session-1")

	// Then the repository delete ran inside the cascade, which owns the
	// knowledge cleanup around it
	require.NoError(t, err)
}

func TestDeleteSession_propagatesSessionNotFound(t *testing.T) {
	// Given a repository with no such session
	sessions := domainstudymocks.NewMockSessionRepository(t)
	cascade := studymocks.NewMockKnowledgeCascade(t)
	sessions.EXPECT().Delete(context.Background(), "missing").Return(domainstudy.ErrSessionNotFound).Once()
	runDeleteThroughCascade(cascade, []string{"missing"})
	service := newDeleteTestService(t, sessions, cascade)

	// When deleting a session that does not exist
	err := service.DeleteSession(context.Background(), "missing")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}

func TestDeleteSession_propagatesTheKnowledgeCascadeError_withoutDeletingTheSession(t *testing.T) {
	// Given a knowledge cascade that refuses to run, e.g. the index is loading
	sessions := domainstudymocks.NewMockSessionRepository(t)
	cascade := studymocks.NewMockKnowledgeCascade(t)
	boom := errors.New("index loading")
	cascade.EXPECT().
		DeleteSessionsWithKnowledge(context.Background(), []string{"session-1"}, mock.Anything).
		Return(boom).Once()
	service := newDeleteTestService(t, sessions, cascade)

	// When deleting the session
	err := service.DeleteSession(context.Background(), "session-1")

	// Then the error propagates; the repository has no expectation, so an
	// unexpected direct delete would fail the test
	require.ErrorIs(t, err, boom)
}
