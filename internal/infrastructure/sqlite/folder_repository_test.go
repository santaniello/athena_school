package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/folder"
)

func newTestFolderRepository(t *testing.T) *FolderRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewFolderRepository(db)
}

func TestFolderRepository_Create_storesFolder(t *testing.T) {
	// Given a repository and a new folder
	repo := newTestFolderRepository(t)
	ctx := context.Background()
	f := folder.Folder{ID: "f-1", Name: "System Design", CreatedAt: time.Now().UTC().Truncate(time.Second)}

	// When creating it
	err := repo.Create(ctx, f)

	// Then it succeeds and is retrievable by ID
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, "f-1")
	require.NoError(t, getErr)
	assert.Equal(t, "System Design", stored.Name)
	assert.False(t, stored.IsDefault)
}

func TestFolderRepository_GetByID_returnsNotFound_whenFolderDoesNotExist(t *testing.T) {
	// Given a repository with no matching folder
	repo := newTestFolderRepository(t)
	ctx := context.Background()

	// When fetching a folder that does not exist
	_, err := repo.GetByID(ctx, "missing")

	// Then it fails with ErrFolderNotFound
	assert.ErrorIs(t, err, folder.ErrFolderNotFound)
}

func TestFolderRepository_List_returnsAllFolders(t *testing.T) {
	// Given a repository with two extra folders, plus the seeded default one
	repo := newTestFolderRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, folder.Folder{ID: "f-1", Name: "System Design"}))
	require.NoError(t, repo.Create(ctx, folder.Folder{ID: "f-2", Name: "Java"}))

	// When listing folders
	folders, err := repo.List(ctx)

	// Then all three are returned
	require.NoError(t, err)
	assert.Len(t, folders, 3)
}

func TestFolderRepository_Rename_updatesName(t *testing.T) {
	// Given a repository with an existing folder
	repo := newTestFolderRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, folder.Folder{ID: "f-1", Name: "Old name"}))

	// When renaming it
	err := repo.Rename(ctx, "f-1", "New name")

	// Then the new name is persisted
	require.NoError(t, err)
	stored, getErr := repo.GetByID(ctx, "f-1")
	require.NoError(t, getErr)
	assert.Equal(t, "New name", stored.Name)
}

func TestFolderRepository_Rename_returnsNotFound_whenFolderDoesNotExist(t *testing.T) {
	// Given a repository with no matching folder
	repo := newTestFolderRepository(t)
	ctx := context.Background()

	// When renaming a folder that does not exist
	err := repo.Rename(ctx, "missing", "New name")

	// Then it fails with ErrFolderNotFound
	assert.ErrorIs(t, err, folder.ErrFolderNotFound)
}

func TestFolderRepository_Delete_removesFolder(t *testing.T) {
	// Given a repository with an existing folder
	repo := newTestFolderRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, folder.Folder{ID: "f-1", Name: "System Design"}))

	// When deleting it
	err := repo.Delete(ctx, "f-1")

	// Then it no longer exists
	require.NoError(t, err)
	_, getErr := repo.GetByID(ctx, "f-1")
	assert.ErrorIs(t, getErr, folder.ErrFolderNotFound)
}

func TestFolderRepository_Delete_returnsNotFound_whenFolderDoesNotExist(t *testing.T) {
	// Given a repository with no matching folder
	repo := newTestFolderRepository(t)
	ctx := context.Background()

	// When deleting a folder that does not exist
	err := repo.Delete(ctx, "missing")

	// Then it fails with ErrFolderNotFound
	assert.ErrorIs(t, err, folder.ErrFolderNotFound)
}

func TestFolderRepository_GetByID_returnsTheSeededDefaultFolder(t *testing.T) {
	// Given a freshly opened database (folders migration seeds "default")
	repo := newTestFolderRepository(t)
	ctx := context.Background()

	// When fetching the default folder
	stored, err := repo.GetByID(ctx, "default")

	// Then it is marked as the default folder
	require.NoError(t, err)
	assert.Equal(t, "General", stored.Name)
	assert.True(t, stored.IsDefault)
}
