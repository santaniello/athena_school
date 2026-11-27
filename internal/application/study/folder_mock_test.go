package study

import (
	"context"
	"testing"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
)

// newFolderMock returns a folder repository whose session folder (the
// zero-value FolderID most test sessions carry) has the Architecture Lab
// enabled, so tests about the chat need no per-test folder setup. The lookup
// is optional: a turn that fails before it never reads it. Tests about a
// disabled folder use newFolderMockWithLab(t, false).
func newFolderMock(t *testing.T) *foldermocks.MockRepository {
	t.Helper()
	return newFolderMockWithLab(t, true)
}

func newFolderMockWithLab(t *testing.T, enabled bool) *foldermocks.MockRepository {
	t.Helper()
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().
		GetByID(context.Background(), "").
		Return(domainfolder.Folder{ArchLabEnabled: enabled}, nil).Maybe()
	return folders
}
