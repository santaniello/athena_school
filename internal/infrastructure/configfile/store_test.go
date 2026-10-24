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

func TestStore_Load_ignoresTheRemovedExtractionLimitKey(t *testing.T) {
	// Given a config file written before the extraction limit was removed
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("openrouter_key: sk-or-legacy\nmax_knowledge_extraction_items: 12\n"), 0o600))
	store := NewStore(path)

	// When loading it
	loaded, err := store.Load()

	// Then it still loads and keeps the key it does understand
	require.NoError(t, err)
	assert.Equal(t, config.Config{OpenRouterKey: "sk-or-legacy"}, loaded)
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

func TestStore_Save_dropsTheRemovedExtractionLimitKeyFromAnOldFile(t *testing.T) {
	// Given a config file that still carries the removed extraction limit
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("openrouter_key: sk-or-old\nmax_knowledge_extraction_items: 12\n"), 0o600))
	store := NewStore(path)
	loaded, err := store.Load()
	require.NoError(t, err)

	// When saving the loaded config back
	require.NoError(t, store.Save(loaded))

	// Then the removed key no longer appears in the file
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "max_knowledge_extraction_items")
	assert.Contains(t, string(data), "openrouter_key: sk-or-old")
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

func fullModelSettings() config.ModelSettings {
	return config.ModelSettings{
		Study:               "anthropic/claude-sonnet-4.5",
		ConversationSummary: "openai/gpt-4o-mini",
		QueryEnrichment:     "openai/gpt-4o-mini",
		NoteFormatting:      "anthropic/claude-sonnet-4.5",
		Embedding:           "openai/text-embedding-3-small",
	}
}

func TestStore_SaveThenLoad_roundTripsTheModelsSection(t *testing.T) {
	// Given a config carrying a key and every model setting
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)
	original := config.Config{OpenRouterKey: "sk-or-abc123", Models: fullModelSettings()}

	// When saving it and loading it back
	require.NoError(t, store.Save(original))
	loaded, err := store.Load()

	// Then nothing is lost
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestStore_Save_writesTheDocumentedModelsKeys(t *testing.T) {
	// Given a store and a config with every model setting
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)

	// When saving it
	require.NoError(t, store.Save(config.Config{OpenRouterKey: "sk-or-abc123", Models: fullModelSettings()}))

	// Then the file uses the documented models section and keys
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	text := string(data)
	assert.Contains(t, text, "models:")
	assert.Contains(t, text, "  study: anthropic/claude-sonnet-4.5")
	assert.Contains(t, text, "  conversation_summary: openai/gpt-4o-mini")
	assert.Contains(t, text, "  query_enrichment: openai/gpt-4o-mini")
	assert.Contains(t, text, "  note_formatting: anthropic/claude-sonnet-4.5")
	assert.Contains(t, text, "  embedding: openai/text-embedding-3-small")
	assert.Contains(t, text, "openrouter_key: sk-or-abc123")
}

func TestStore_Save_omitsEmptyModelSettings(t *testing.T) {
	// Given a config with only one model setting
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)

	// When saving it
	require.NoError(t, store.Save(config.Config{OpenRouterKey: "k", Models: config.ModelSettings{Study: "a/b"}}))

	// Then the unset keys are not written, so absent still means the default
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	assert.Contains(t, string(data), "study: a/b")
	assert.NotContains(t, string(data), "embedding")
	assert.NotContains(t, string(data), "note_formatting")
}

func TestStore_Save_writesNoModelsSectionWhenNothingIsSet(t *testing.T) {
	// Given a config with a key and no model settings
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := NewStore(path)

	// When saving it
	require.NoError(t, store.Save(config.Config{OpenRouterKey: "k"}))

	// Then the file stays exactly as before this feature
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "models")
}

func TestStore_Load_legacyFileWithOnlyTheKeyHasNoModelSettings(t *testing.T) {
	// Given a config file written before models existed
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("openrouter_key: sk-or-legacy\n"), 0o600))

	// When loading it
	loaded, err := NewStore(path).Load()

	// Then every model setting is empty, meaning the defaults
	require.NoError(t, err)
	assert.Equal(t, config.Config{OpenRouterKey: "sk-or-legacy"}, loaded)
	assert.Equal(t, config.ModelSettings{}, loaded.Models)
}
