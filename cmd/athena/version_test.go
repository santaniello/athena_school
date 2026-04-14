package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenVersionArgs_WhenVersionCommandExecuted_ThenPrintsV010(t *testing.T) {
	// Given: a captured output buffer and version args
	buf := &bytes.Buffer{}

	// When: executing the version subcommand
	err := execute([]string{"version"}, buf)

	// Then: no error and output contains the version string
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "v0.1.0")
}

func Test_GivenFreshRootCmd_WhenVersionSubcommandInspected_ThenHasNonEmptyShortDescription(t *testing.T) {
	// Given: a fresh root command
	cmd := buildRootCmd()

	// When: looking up the version subcommand
	var versionShort string
	for _, sub := range cmd.Commands() {
		if sub.Name() == "version" {
			versionShort = sub.Short
		}
	}

	// Then: it has a non-empty short description
	assert.NotEmpty(t, versionShort)
}