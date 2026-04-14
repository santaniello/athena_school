package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenHelpFlag_WhenRootCommandExecuted_ThenPrintsToolName(t *testing.T) {
	// Given: a captured output buffer and --help args
	buf := &bytes.Buffer{}

	// When: executing the root command with --help
	err := execute([]string{"--help"}, buf)

	// Then: no error and output contains the tool name
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "athena")
}

func Test_GivenFreshRootCmd_WhenInspectingSubcommands_ThenVersionIsPresent(t *testing.T) {
	// Given: a fresh root command
	cmd := buildRootCmd()

	// When: inspecting its subcommands
	subcommandNames := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		subcommandNames = append(subcommandNames, sub.Name())
	}

	// Then: version subcommand is present
	assert.Contains(t, subcommandNames, "version")
}

func Test_GivenHelpArgs_WhenExecuteCalled_ThenNoError(t *testing.T) {
	// Given: a captured output buffer and --help args
	buf := &bytes.Buffer{}

	// When: calling execute with --help (success path, no os.Exit)
	err := execute([]string{"--help"}, buf)

	// Then: no error
	require.NoError(t, err)
}

func Test_GivenHelpFlag_WhenExecutePublicEntryPoint_ThenPrintsToolName(t *testing.T) {
	// Given: os.Args set to --help and stdout redirected to discard output
	oldArgs := os.Args
	os.Args = []string{"athena", "--help"}
	defer func() { os.Args = oldArgs }()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	// When: calling Execute (public entry point, success path — no os.Exit expected)
	Execute()

	// Then: stdout received content (help text) without panicking
	w.Close()
	var buf bytes.Buffer
	_, readErr := buf.ReadFrom(r)
	require.NoError(t, readErr)
	assert.Contains(t, buf.String(), "athena")
}