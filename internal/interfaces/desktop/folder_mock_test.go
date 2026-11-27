package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
)

// newFolderMock returns a folder repository whose folders all have the
// Architecture Lab enabled; the lookup the study chat makes is optional, so a
// test that fails before it needs no setup.
func newFolderMock(t *testing.T) *foldermocks.MockRepository {
	t.Helper()
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().
		GetByID(mock.MatchedBy(func(context.Context) bool { return true }), mock.MatchedBy(func(string) bool { return true })).
		Return(domainfolder.Folder{ArchLabEnabled: true}, nil).Maybe()
	return folders
}
