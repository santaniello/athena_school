package desktop

import (
	"context"
	"testing"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
)

// newFolderMock returns a folder repository whose session folder (the
// zero-value FolderID the test sessions carry) has the Architecture Lab
// enabled; the lookup the study chat makes is optional, so a
// test that fails before it needs no setup.
func newFolderMock(t *testing.T) *foldermocks.MockRepository {
	t.Helper()
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().
		GetByID(context.Background(), "").
		Return(domainfolder.Folder{ArchLabEnabled: true}, nil).Maybe()
	return folders
}
