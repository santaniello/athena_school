package folder

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestDeleteFolder_deletesSessionsBeforeDeletingFolder(t *testing.T) {
	// Given a service tracking the order ports are called in
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)

	var callOrder []string
	sessions.EXPECT().
		DeleteByFolder(context.Background(), "f-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete-sessions") }).
		Return(nil).
		Once()
	folders.EXPECT().
		Delete(context.Background(), "f-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete-folder") }).
		Return(nil).
		Once()
	service := NewService(folders, sessions)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then its sessions are deleted before the folder itself
	require.NoError(t, err)
	require.Equal(t, []string{"delete-sessions", "delete-folder"}, callOrder)
}

func TestDeleteFolder_doesNotDeleteFolder_whenDeletingSessionsFails(t *testing.T) {
	// Given a service whose session deletion fails
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	deleteSessionsErr := errors.New("boom")
	sessions.EXPECT().
		DeleteByFolder(context.Background(), "f-1").
		Return(deleteSessionsErr).
		Once()
	service := NewService(folders, sessions)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then the DeleteByFolder error propagates, wrapped; folders.Delete has
	// no .EXPECT(), so an unexpected call would fail the test
	require.ErrorIs(t, err, deleteSessionsErr)
}
