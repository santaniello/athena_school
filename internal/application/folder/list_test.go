package folder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestListFolders_returnsEveryFolder(t *testing.T) {
	// Given a repository with two folders
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	want := []domainfolder.Folder{
		{ID: "default", Name: "General", IsDefault: true},
		{ID: "f-1", Name: "System Design"},
	}
	folders.EXPECT().List(context.Background()).Return(want, nil).Once()
	service := NewService(folders, sessions)

	// When listing folders
	got, err := service.ListFolders(context.Background())

	// Then every folder is returned
	require.NoError(t, err)
	require.Equal(t, want, got)
}
