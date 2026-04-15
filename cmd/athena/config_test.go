package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenNoArgs_WhenConfigGet_ThenPrintsCurrentConfig(t *testing.T) {
	// Given: a temp config path with defaults
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: executing config get
	err := execute([]string{"--config", configPath, "config", "get"}, buf)

	// Then: prints default provider and model
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "provider: ollama")
	assert.Contains(t, output, "model:    llama3")
}

func Test_GivenProviderArg_WhenConfigSetProvider_ThenPrintsConfirmation(t *testing.T) {
	// Given: a temp config path
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: executing config set provider ollama
	err := execute([]string{"--config", configPath, "config", "set", "provider", "ollama"}, buf)

	// Then: prints confirmation and persists the value
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `provider set to "ollama"`)

	_, statErr := os.Stat(configPath)
	assert.NoError(t, statErr)
}

func Test_GivenModelArg_WhenConfigSetModel_ThenPrintsConfirmation(t *testing.T) {
	// Given: a temp config path
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: executing config set model llama3
	err := execute([]string{"--config", configPath, "config", "set", "model", "llama3"}, buf)

	// Then: prints confirmation
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `model set to "llama3"`)
}

func Test_GivenOllamaHostArg_WhenConfigSetOllamaHost_ThenPrintsConfirmation(t *testing.T) {
	// Given: a temp config path
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: executing config set ollama.host
	err := execute([]string{"--config", configPath, "config", "set", "ollama.host", "http://localhost:11434"}, buf)

	// Then: prints confirmation
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `ollama.host set to "http://localhost:11434"`)
}

func Test_GivenUnknownKey_WhenConfigSet_ThenReturnsError(t *testing.T) {
	// Given: a temp config path and an unknown key
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: executing config set with an unknown key
	err := execute([]string{"--config", configPath, "config", "set", "unknown", "value"}, buf)

	// Then: returns an error
	assert.Error(t, err)
}

func Test_GivenSetProvider_WhenConfigGet_ThenReflectsUpdatedValue(t *testing.T) {
	// Given: a temp config path with provider already saved
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	setErr := execute([]string{"--config", configPath, "config", "set", "provider", "openai"}, &bytes.Buffer{})
	require.NoError(t, setErr)

	buf := &bytes.Buffer{}

	// When: running config get
	err := execute([]string{"--config", configPath, "config", "get"}, buf)

	// Then: reflects the updated provider
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "provider: openai")
}
