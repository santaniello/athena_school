package folder

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSetArchLabEnabled_storesTheFlag(t *testing.T) {
	// Given a service whose repository accepts the change
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().SetArchLabEnabled(context.Background(), "f-1", true).Return(nil).Once()
	service := NewService(folders, studymocks.NewMockSessionRepository(t), nil)

	// When enabling the Architecture Lab
	err := service.SetArchLabEnabled(context.Background(), "f-1", true)

	// Then it succeeds
	require.NoError(t, err)
}

func TestSetArchLabEnabled_propagatesFolderNotFound(t *testing.T) {
	// Given a repository with no such folder
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().SetArchLabEnabled(context.Background(), "missing", false).Return(domainfolder.ErrFolderNotFound).Once()
	service := NewService(folders, studymocks.NewMockSessionRepository(t), nil)

	// When disabling it
	err := service.SetArchLabEnabled(context.Background(), "missing", false)

	// Then ErrFolderNotFound surfaces
	assert.ErrorIs(t, err, domainfolder.ErrFolderNotFound)
}

func TestSetArchLabEnabled_wrapsRepositoryFailure(t *testing.T) {
	// Given a failing repository
	boom := errors.New("disk full")
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().SetArchLabEnabled(context.Background(), "f-1", true).Return(boom).Once()
	service := NewService(folders, studymocks.NewMockSessionRepository(t), nil)

	// When enabling the lab
	err := service.SetArchLabEnabled(context.Background(), "f-1", true)

	// Then the cause is kept
	assert.ErrorIs(t, err, boom)
}

func TestListFolders_returnsTheArchLabFlag(t *testing.T) {
	// Given a repository with one enabled and one disabled folder
	want := []domainfolder.Folder{{ID: "f-1", Name: "Design", ArchLabEnabled: true}, {ID: "f-2", Name: "Java"}}
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().List(context.Background()).Return(want, nil).Once()
	service := NewService(folders, studymocks.NewMockSessionRepository(t), nil)

	// When listing
	got, err := service.ListFolders(context.Background())

	// Then both flags are returned
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
