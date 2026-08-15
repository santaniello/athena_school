package profilefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/profile"
)

func TestStore_SaveThenLoad_roundTrips(t *testing.T) {
	// Given a store pointing at a profile file and a profile to persist
	path := filepath.Join(t.TempDir(), "profile.json")
	store := NewStore(path)
	original := profile.UserProfile{
		Name:              "Ana",
		AssistantName:     "Atena",
		Area:              "Engenharia de Software",
		ExperienceLevel:   profile.ExperienceLevelIntermediate,
		Goals:             []string{"SQL", "System Design"},
		StudyStyle:        profile.StudyStylePracticalExamples,
		AssistantLanguage: profile.AssistantLanguageEnglish,
		CreatedAt:         time.Now().UTC().Truncate(time.Second),
	}

	// When saving it and loading it back
	require.NoError(t, store.Save(original))
	loaded, err := store.Load()

	// Then the loaded profile matches what was saved
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestStore_Save_writesFileWithOwnerOnlyPermissions(t *testing.T) {
	// Given a store pointing at a profile file
	path := filepath.Join(t.TempDir(), "profile.json")
	store := NewStore(path)

	// When saving a profile
	require.NoError(t, store.Save(profile.UserProfile{Name: "Ana"}))

	// Then the file is written with owner-only permissions
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestStore_Save_createsMissingParentDirectory(t *testing.T) {
	// Given a store whose target directory does not exist yet
	path := filepath.Join(t.TempDir(), "nested", "profile.json")
	store := NewStore(path)

	// When saving a profile
	err := store.Save(profile.UserProfile{Name: "Ana"})

	// Then it succeeds and the file exists
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func TestStore_Load_returnsErrorWhenNoProfileExists(t *testing.T) {
	// Given a store pointing at a profile file that was never saved
	store := NewStore(filepath.Join(t.TempDir(), "profile.json"))

	// When loading it
	_, err := store.Load()

	// Then it returns an error instead of a zero-value profile
	assert.Error(t, err)
}
