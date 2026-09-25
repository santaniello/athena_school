package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/folder"
	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func newTestFolderApp(t *testing.T, folders domainfolder.Repository, sessions domainstudy.SessionRepository) *App {
	t.Helper()
	folderService := folder.NewService(folders, sessions, passthroughKnowledgeCascade{})
	app := NewApp(nil, nil, nil, nil, folderService, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	return app
}

func TestApp_CreateFolder_createsAndReturnsFolder(t *testing.T) {
	// Given an App backed by a folder repository that accepts a new folder
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	folders.EXPECT().Create(mock.Anything, mock.AnythingOfType("folder.Folder")).Return(nil).Once()
	app := newTestFolderApp(t, folders, sessions)

	// When creating a folder
	result, err := app.CreateFolder("System Design")

	// Then it returns the created folder
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "System Design", result.Name)
}

func TestApp_CreateFolder_propagatesNameRequiredError(t *testing.T) {
	// Given an App backed by a folder repository
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	app := newTestFolderApp(t, folders, sessions)

	// When creating a folder with a blank name
	_, err := app.CreateFolder("   ")

	// Then the error propagates; the repository is never touched
	require.ErrorIs(t, err, domainfolder.ErrNameRequired)
}

func TestApp_RenameFolder_renamesFolder(t *testing.T) {
	// Given an App backed by a folder repository that accepts the rename
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	folders.EXPECT().Rename(mock.Anything, "f-1", "New name").Return(nil).Once()
	app := newTestFolderApp(t, folders, sessions)

	// When renaming a folder
	err := app.RenameFolder("f-1", "New name")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_DeleteFolder_deletesSessionsThenDeletesFolder(t *testing.T) {
	// Given an App backed by ports that accept the session delete and folder delete
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	sessions.EXPECT().ListByFolder(mock.Anything, "f-1").Return([]domainstudy.Session{{ID: "s-1"}}, nil).Once()
	sessions.EXPECT().DeleteByFolder(mock.Anything, "f-1").Return(nil).Once()
	folders.EXPECT().Delete(mock.Anything, "f-1").Return(nil).Once()
	app := newTestFolderApp(t, folders, sessions)

	// When deleting a folder
	err := app.DeleteFolder("f-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_ListFolders_returnsEveryFolder(t *testing.T) {
	// Given an App backed by a repository with two folders
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	folders.EXPECT().List(mock.Anything).Return([]domainfolder.Folder{
		{ID: "f-0", Name: "General"},
		{ID: "f-1", Name: "System Design"},
	}, nil).Once()
	app := newTestFolderApp(t, folders, sessions)

	// When listing folders
	results, err := app.ListFolders()

	// Then every folder is returned as a DTO
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "General", results[0].Name)
	assert.Equal(t, "System Design", results[1].Name)
}
