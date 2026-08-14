package athenahome

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir_createsAthenaDirectoryUnderHome(t *testing.T) {
	// Given a HOME pointing at an empty temp directory
	home := t.TempDir()
	t.Setenv("HOME", home)

	// When resolving the Athena directory
	dir, err := Dir()

	// Then it is created under HOME with owner-only permissions
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".athena"), dir)
	info, statErr := os.Stat(dir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestDir_isIdempotentWhenDirectoryAlreadyExists(t *testing.T) {
	// Given a HOME whose Athena directory was already resolved once
	home := t.TempDir()
	t.Setenv("HOME", home)
	first, err := Dir()
	require.NoError(t, err)

	// When resolving it again
	second, err := Dir()

	// Then it succeeds and returns the same path without error
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestFile_returnsPathInsideAthenaDir(t *testing.T) {
	// Given a HOME pointing at an empty temp directory
	home := t.TempDir()
	t.Setenv("HOME", home)

	// When resolving a file name inside the Athena directory
	path, err := File("session.json")

	// Then the path is the file joined under ~/.athena
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".athena", "session.json"), path)
}

func TestFile_rejectsNameThatEscapesAthenaDir(t *testing.T) {
	// Given a HOME pointing at an empty temp directory
	home := t.TempDir()
	t.Setenv("HOME", home)

	// When resolving a file name that traverses outside ~/.athena
	_, err := File("../evil")

	// Then it is rejected instead of returning an escaped path
	assert.Error(t, err)
}
