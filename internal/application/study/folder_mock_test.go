package study

import (
	"testing"

	"github.com/stretchr/testify/mock"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"

	"context"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
)

// newFolderMock returns a folder repository whose folders all have the
// Architecture Lab enabled, so tests about the chat need no per-test folder
// setup. The lookup is optional: a turn that fails before it never reads it.
// Tests about a disabled folder use newFolderMockWithLab(t, false).
func newFolderMock(t *testing.T) *foldermocks.MockRepository {
	t.Helper()
	return newFolderMockWithLab(t, true)
}

func newFolderMockWithLab(t *testing.T, enabled bool) *foldermocks.MockRepository {
	t.Helper()
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().
		GetByID(mock.MatchedBy(func(context.Context) bool { return true }), mock.MatchedBy(func(string) bool { return true })).
		Return(domainfolder.Folder{ArchLabEnabled: enabled}, nil).Maybe()
	return folders
}
