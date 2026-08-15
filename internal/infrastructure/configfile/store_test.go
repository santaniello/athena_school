package configfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/config"
)

func TestStore_SaveThenLoad_roundTrips(t *testing.T) {
	// Given a store pointing at a config file and a config to persist
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)
	original := config.Config{OpenRouterKey: "sk-or-abc123"}

	// When saving it and loading it back
	require.NoError(t, store.Save(original))
	loaded, err := store.Load()

	// Then the loaded config matches what was saved
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestStore_Save_writesOpenRouterKeyAsYAML(t *testing.T) {
	// Given a store pointing at a config file
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)

	// When saving a config
	require.NoError(t, store.Save(config.Config{OpenRouterKey: "sk-or-abc123"}))

	// Then the file on disk uses the documented openrouter_key YAML field
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	assert.Contains(t, string(data), "openrouter_key: sk-or-abc123")
}

func TestStore_Save_writesFileWithOwnerOnlyPermissions(t *testing.T) {
	// Given a store pointing at a config file
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)

	// When saving a config
	require.NoError(t, store.Save(config.Config{OpenRouterKey: "sk-or-abc123"}))

	// Then the file is written with owner-only permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestStore_Save_createsMissingParentDirectory(t *testing.T) {
	// Given a store whose target directory does not exist yet
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	store := NewStore(path)

	// When saving a config
	err := store.Save(config.Config{OpenRouterKey: "sk-or-abc123"})

	// Then it succeeds and the file exists
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func TestStore_Load_returnsErrorWhenNoConfigExists(t *testing.T) {
	// Given a store pointing at a config file that was never saved
	store := NewStore(filepath.Join(t.TempDir(), "config.yaml"))

	// When loading it
	_, err := store.Load()

	// Then it returns an error instead of a zero-value config
	assert.Error(t, err)
}
