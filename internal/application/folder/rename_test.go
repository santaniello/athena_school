package folder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestRenameFolder_returnsNameRequired_whenNameIsBlank(t *testing.T) {
	// Given a service and a blank name
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	service := NewService(folders, sessions)

	// When renaming a folder with a whitespace-only name
	err := service.RenameFolder(context.Background(), "f-1", "   ")

	// Then it fails with ErrNameRequired; no port received any call
	require.ErrorIs(t, err, domainfolder.ErrNameRequired)
}

func TestRenameFolder_renamesFolder(t *testing.T) {
	// Given a service whose repository accepts the rename
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	folders.EXPECT().Rename(context.Background(), "f-1", "New name").Return(nil).Once()
	service := NewService(folders, sessions)

	// When renaming it, including the default folder — only delete is
	// blocked for the default folder, not rename
	err := service.RenameFolder(context.Background(), "f-1", "New name")

	// Then it succeeds
	require.NoError(t, err)
}

func TestRenameFolder_propagatesFolderNotFound(t *testing.T) {
	// Given a service backed by a repository with no such folder
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	folders.EXPECT().Rename(context.Background(), "missing", "New name").Return(domainfolder.ErrFolderNotFound).Once()
	service := NewService(folders, sessions)

	// When renaming a folder that does not exist
	err := service.RenameFolder(context.Background(), "missing", "New name")

	// Then the error propagates
	require.ErrorIs(t, err, domainfolder.ErrFolderNotFound)
}
