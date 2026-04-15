package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GivenFreshRootCmd_WhenInspected_ThenStudySubcommandIsPresent(t *testing.T) {
	// Given: a freshly built root command
	cmd := buildRootCmd()

	// When: collecting the names of all subcommands
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}

	// Then: "study" is among the registered subcommands
	assert.Contains(t, names, "study")
}

func Test_GivenStudySubcommand_WhenInspected_ThenHasNonEmptyShortDescription(t *testing.T) {
	// Given: the root command with all subcommands registered
	cmd := buildRootCmd()

	// When: finding the study subcommand
	var short string
	for _, sub := range cmd.Commands() {
		if sub.Name() == "study" {
			short = sub.Short
		}
	}

	// Then: the short description is non-empty
	assert.NotEmpty(t, short)
}

func Test_GivenStudyCommand_WhenCalledWithNoTopic_ThenReturnsError(t *testing.T) {
	// Given: no topic argument
	buf := &bytes.Buffer{}

	// When: running "study" with no positional arguments
	err := execute([]string{"study"}, buf)

	// Then: cobra returns an argument validation error
	assert.Error(t, err)
}

func Test_GivenUnknownProvider_WhenStudyRun_ThenReturnsProviderError(t *testing.T) {
	// Given: a temp config and an unknown provider flag
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: running study with an unknown provider
	err := execute([]string{"--config", configPath, "study", "caching", "--provider", "unknown-provider"}, buf)

	// Then: an error about the unknown provider is returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-provider")
}

func Test_GivenTopicAndSubtopicWithUnknownProvider_WhenStudyRun_ThenReturnsError(t *testing.T) {
	// Given: a temp config, topic, subtopic, and an unknown provider
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: running study with two positional args and an unknown provider
	err := execute([]string{"--config", configPath, "study", "system-design", "caching", "--provider", "unknown-provider"}, buf)

	// Then: an error about the unknown provider is returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-provider")
}

func Test_GivenModelFlag_WhenStudyRunWithUnknownProvider_ThenReturnsError(t *testing.T) {
	// Given: a temp config with model and provider flags
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	buf := &bytes.Buffer{}

	// When: running study with both --model and an unknown --provider
	err := execute([]string{"--config", configPath, "study", "caching", "--provider", "unknown-provider", "--model", "llama3"}, buf)

	// Then: an error about the unknown provider is returned (model flag was processed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown-provider")
}
