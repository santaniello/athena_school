package folder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestCreateFolder_returnsNameRequired_whenNameIsBlank(t *testing.T) {
	// Given a service and a blank name
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	service := NewService(folders, sessions)

	// When creating a folder with a whitespace-only name
	_, err := service.CreateFolder(context.Background(), "   ")

	// Then it fails with ErrNameRequired; no port received any call
	require.ErrorIs(t, err, domainfolder.ErrNameRequired)
}

func TestCreateFolder_createsAndPersistsFolder(t *testing.T) {
	// Given a service whose repository accepts a new folder
	folders := foldermocks.NewMockRepository(t)
	sessions := studymocks.NewMockSessionRepository(t)
	folders.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(f domainfolder.Folder) bool {
			return f.ID != "" && f.Name == "System Design" && !f.IsDefault && !f.CreatedAt.IsZero()
		})).
		Return(nil).
		Once()
	service := NewService(folders, sessions)

	// When creating a folder
	f, err := service.CreateFolder(context.Background(), "System Design")

	// Then it succeeds and returns the created folder
	require.NoError(t, err)
	require.NotEmpty(t, f.ID)
	require.Equal(t, "System Design", f.Name)
}
