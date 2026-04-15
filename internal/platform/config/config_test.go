package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsantaniello/athena_school/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenNoConfigFile_WhenLoad_ThenReturnsDefaults(t *testing.T) {
	// Given:
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// When:
	cfg, err := config.Load(path)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, config.DefaultProvider, cfg.Provider)
	assert.Equal(t, config.DefaultModel, cfg.Model)
	assert.Equal(t, config.DefaultOllamaHost, cfg.Ollama.Host)
}

func Test_GivenValidConfigFile_WhenLoad_ThenReturnsFileValues(t *testing.T) {
	// Given:
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlContent := "provider: openai\nmodel: gpt-4\nollama:\n  host: http://remote:11434\n"
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0600))

	// When:
	cfg, err := config.Load(path)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "openai", cfg.Provider)
	assert.Equal(t, "gpt-4", cfg.Model)
	assert.Equal(t, "http://remote:11434", cfg.Ollama.Host)
}

func Test_GivenConfig_WhenSave_ThenWritesYAMLToDisk(t *testing.T) {
	// Given:
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Provider: "ollama",
		Model:    "llama3",
		Ollama:   config.OllamaConfig{Host: "http://localhost:11434"},
	}

	// When:
	err := config.Save(path, cfg)

	// Then:
	require.NoError(t, err)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	content := string(data)
	assert.Contains(t, content, "provider: ollama")
	assert.Contains(t, content, "model: llama3")
	assert.Contains(t, content, "host: http://localhost:11434")
}

func Test_GivenMissingDirectory_WhenSave_ThenCreatesDirectoryAndFile(t *testing.T) {
	// Given:
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "config.yaml")
	cfg := &config.Config{
		Provider: "ollama",
		Model:    "llama3",
		Ollama:   config.OllamaConfig{Host: "http://localhost:11434"},
	}

	// When:
	err := config.Save(path, cfg)

	// Then:
	require.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func Test_GivenSavedConfig_WhenLoad_ThenRoundTripsCorrectly(t *testing.T) {
	// Given:
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := &config.Config{
		Provider: "openai",
		Model:    "gpt-4",
		Ollama:   config.OllamaConfig{Host: "http://remote:11434"},
	}
	require.NoError(t, config.Save(path, original))

	// When:
	loaded, err := config.Load(path)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, original.Provider, loaded.Provider)
	assert.Equal(t, original.Model, loaded.Model)
	assert.Equal(t, original.Ollama.Host, loaded.Ollama.Host)
}

func Test_GivenInvalidYAML_WhenLoad_ThenReturnsError(t *testing.T) {
	// Given: a config file with invalid YAML content
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(":\tinvalid: yaml: {"), 0600))

	// When: loading the config
	cfg, err := config.Load(path)

	// Then: returns an error and no config
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func Test_GivenDefaultPath_WhenCalled_ThenReturnsXDGPath(t *testing.T) {
	// Given: a valid home directory environment

	// When: calling DefaultPath
	path, err := config.DefaultPath()

	// Then: returns a non-empty path ending in the expected suffix
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.True(t, filepath.IsAbs(path))
	assert.Contains(t, path, filepath.Join(".config", "athena", "config.yaml"))
}

func Test_GivenSavedFile_WhenSave_ThenFilePermissionsAre0600(t *testing.T) {
	// Given:
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := &config.Config{
		Provider: "ollama",
		Model:    "llama3",
		Ollama:   config.OllamaConfig{Host: "http://localhost:11434"},
	}

	// When:
	err := config.Save(path, cfg)

	// Then:
	require.NoError(t, err)
	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
