package folder

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	foldermocks "github.com/santaniello/athena/internal/application/folder/mocks"
	domainfoldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestDeleteFolder_deletesItsSessionsWithTheirKnowledge_thenTheFolder(t *testing.T) {
	// Given a folder holding two sessions and a service tracking call order
	folders := domainfoldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	cascade := foldermocks.NewMockKnowledgeCascade(t)
	var callOrder []string
	sessions.EXPECT().ListByFolder(context.Background(), "f-1").
		Return([]domainstudy.Session{{ID: "s-1"}, {ID: "s-2"}}, nil).Once()
	cascade.EXPECT().
		DeleteSessionsWithKnowledge(context.Background(), []string{"s-1", "s-2"}, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ []string, deleteSessions func(context.Context) error) error {
			callOrder = append(callOrder, "cascade")
			return deleteSessions(ctx)
		}).Once()
	sessions.EXPECT().DeleteByFolder(context.Background(), "f-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete-sessions") }).
		Return(nil).Once()
	folders.EXPECT().Delete(context.Background(), "f-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete-folder") }).
		Return(nil).Once()
	service := NewService(folders, sessions, cascade)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then the sessions go inside the knowledge cascade, before the folder
	require.NoError(t, err)
	require.Equal(t, []string{"cascade", "delete-sessions", "delete-folder"}, callOrder)
}

func TestDeleteFolder_doesNotDeleteFolder_whenDeletingSessionsFails(t *testing.T) {
	// Given a session deletion that fails inside the cascade
	folders := domainfoldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	cascade := foldermocks.NewMockKnowledgeCascade(t)
	deleteSessionsErr := errors.New("boom")
	sessions.EXPECT().ListByFolder(context.Background(), "f-1").Return([]domainstudy.Session{{ID: "s-1"}}, nil).Once()
	cascade.EXPECT().
		DeleteSessionsWithKnowledge(context.Background(), []string{"s-1"}, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ []string, deleteSessions func(context.Context) error) error {
			return deleteSessions(ctx)
		}).Once()
	sessions.EXPECT().DeleteByFolder(context.Background(), "f-1").Return(deleteSessionsErr).Once()
	service := NewService(folders, sessions, cascade)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then the error propagates; folders.Delete has no expectation, so an
	// unexpected call would fail the test
	require.ErrorIs(t, err, deleteSessionsErr)
}

func TestDeleteFolder_doesNotDeleteAnything_whenListingItsSessionsFails(t *testing.T) {
	// Given a session listing that fails
	folders := domainfoldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	listErr := errors.New("boom")
	sessions.EXPECT().ListByFolder(context.Background(), "f-1").Return(nil, listErr).Once()
	service := NewService(folders, sessions, foldermocks.NewMockKnowledgeCascade(t))

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then the error propagates and no session, knowledge or folder is touched
	require.ErrorIs(t, err, listErr)
}

func TestDeleteFolder_propagatesTheKnowledgeCascadeError_withoutDeletingTheFolder(t *testing.T) {
	// Given a knowledge cascade that refuses to run
	folders := domainfoldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	cascade := foldermocks.NewMockKnowledgeCascade(t)
	boom := errors.New("index loading")
	sessions.EXPECT().ListByFolder(context.Background(), "f-1").Return([]domainstudy.Session{{ID: "s-1"}}, nil).Once()
	cascade.EXPECT().
		DeleteSessionsWithKnowledge(context.Background(), []string{"s-1"}, mock.Anything).
		Return(boom).Once()
	service := NewService(folders, sessions, cascade)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then the error propagates and the folder is kept
	require.ErrorIs(t, err, boom)
}
