package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/auth"
)

func TestStore_SaveThenLoad_roundTrips(t *testing.T) {
	// Given a store pointing at a session file and a session to persist
	path := filepath.Join(t.TempDir(), "session.json")
	store := NewStore(path)
	original := auth.Session{
		AccountID: "acc-1",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}

	// When saving it and loading it back
	require.NoError(t, store.Save(original))
	loaded, err := store.Load()

	// Then the loaded session matches what was saved
	require.NoError(t, err)
	assert.Equal(t, original.AccountID, loaded.AccountID)
	assert.True(t, original.CreatedAt.Equal(loaded.CreatedAt))
}

func TestStore_Save_writesFileWithOwnerOnlyPermissions(t *testing.T) {
	// Given a store pointing at a session file
	path := filepath.Join(t.TempDir(), "session.json")
	store := NewStore(path)

	// When saving a session
	require.NoError(t, store.Save(auth.Session{AccountID: "acc-1", CreatedAt: time.Now()}))

	// Then the file is written with owner-only permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestStore_Save_createsMissingParentDirectory(t *testing.T) {
	// Given a store whose target directory does not exist yet
	path := filepath.Join(t.TempDir(), "nested", "session.json")
	store := NewStore(path)

	// When saving a session
	err := store.Save(auth.Session{AccountID: "acc-1", CreatedAt: time.Now()})

	// Then it succeeds and the file exists
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func TestStore_Load_returnsErrorWhenNoSessionExists(t *testing.T) {
	// Given a store pointing at a session file that was never saved
	store := NewStore(filepath.Join(t.TempDir(), "session.json"))

	// When loading it
	_, err := store.Load()

	// Then it returns an error instead of a zero-value session
	assert.Error(t, err)
}
