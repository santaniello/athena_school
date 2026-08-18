package folder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestDeleteFolder_returnsCannotDeleteDefaultFolder_whenIDIsDefault(t *testing.T) {
	// Given a service and the default folder's ID
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	service := NewService(folders, sessions)

	// When deleting the default folder
	err := service.DeleteFolder(context.Background(), domainfolder.DefaultFolderID)

	// Then it fails with ErrCannotDeleteDefaultFolder; no port received any call
	require.ErrorIs(t, err, domainfolder.ErrCannotDeleteDefaultFolder)
}

func TestDeleteFolder_reassignsSessionsToDefaultFolderBeforeDeleting(t *testing.T) {
	// Given a service tracking the order ports are called in
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)

	var callOrder []string
	sessions.EXPECT().
		ReassignFolder(context.Background(), "f-1", domainfolder.DefaultFolderID).
		Run(func(context.Context, string, string) { callOrder = append(callOrder, "reassign") }).
		Return(nil).
		Once()
	folders.EXPECT().
		Delete(context.Background(), "f-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "delete") }).
		Return(nil).
		Once()
	service := NewService(folders, sessions)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then its sessions are reassigned to the default folder before it is deleted
	require.NoError(t, err)
	require.Equal(t, []string{"reassign", "delete"}, callOrder)
}

func TestDeleteFolder_doesNotDeleteFolder_whenReassignFails(t *testing.T) {
	// Given a service whose reassignment fails
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	sessions.EXPECT().
		ReassignFolder(context.Background(), "f-1", domainfolder.DefaultFolderID).
		Return(domainfolder.ErrFolderNotFound).
		Once()
	service := NewService(folders, sessions)

	// When deleting the folder
	err := service.DeleteFolder(context.Background(), "f-1")

	// Then the error propagates; folders.Delete has no .EXPECT(), so an
	// unexpected call would fail the test
	require.Error(t, err)
}
