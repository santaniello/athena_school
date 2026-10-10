package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestApp_DownloadExampleNote_usesTheExactTitleAndDefaultFilename(t *testing.T) {
	// Given an App whose save-file dialog captures the options it was called with
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	dir := t.TempDir()
	chosenPath := filepath.Join(dir, "example-note.md")
	var captured wailsruntime.SaveDialogOptions
	app.saveFile = func(_ context.Context, options wailsruntime.SaveDialogOptions) (string, error) {
		captured = options
		return chosenPath, nil
	}

	// When downloading the example note
	path, err := app.DownloadExampleNote()

	// Then the chosen path is returned, and the dialog was configured with
	// the exact title, default filename and a markdown-only filter
	require.NoError(t, err)
	assert.Equal(t, chosenPath, path)
	assert.Equal(t, "Save example note", captured.Title)
	assert.Equal(t, "example-note.md", captured.DefaultFilename)
	require.Len(t, captured.Filters, 1)
	assert.Equal(t, "Markdown (*.md)", captured.Filters[0].DisplayName)
	assert.Equal(t, "*.md", captured.Filters[0].Pattern)
}

func TestApp_DownloadExampleNote_writesTheExactExampleContent_toTheChosenPath(t *testing.T) {
	// Given a save dialog that resolves to a path in a real temp directory
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	dir := t.TempDir()
	chosenPath := filepath.Join(dir, "example-note.md")
	app.saveFile = func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return chosenPath, nil
	}

	// When downloading the example note
	path, err := app.DownloadExampleNote()

	// Then the exact bundled example content was written to that path
	require.NoError(t, err)
	assert.Equal(t, chosenPath, path)
	written, readErr := os.ReadFile(filepath.Clean(chosenPath))
	require.NoError(t, readErr)
	assert.Equal(t, exampleNoteMarkdown, string(written))
}

func TestApp_DownloadExampleNote_returnsError_whenDialogFails(t *testing.T) {
	// Given a save dialog that fails
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	boom := errors.New("dialog unavailable")
	app.saveFile = func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return "", boom
	}

	// When downloading the example note
	_, err := app.DownloadExampleNote()

	// Then the error propagates, and nothing was written anywhere (there is
	// no path to write to)
	assert.ErrorIs(t, err, boom)
}

func TestApp_DownloadExampleNote_returnsEmptyPath_onCancellation(t *testing.T) {
	// Given a save dialog the user cancelled
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	app.saveFile = func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return "", nil
	}

	// When downloading the example note
	path, err := app.DownloadExampleNote()

	// Then no error occurs and the empty path passes through as-is, with
	// no write attempted
	require.NoError(t, err)
	assert.Empty(t, path)
}

func TestApp_DownloadExampleNote_returnsError_whenWriteFails(t *testing.T) {
	// Given a save dialog that resolves to a path inside a directory that
	// does not exist, so the write itself fails
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	chosenPath := filepath.Join(t.TempDir(), "does-not-exist", "example-note.md")
	app.saveFile = func(context.Context, wailsruntime.SaveDialogOptions) (string, error) {
		return chosenPath, nil
	}

	// When downloading the example note
	_, err := app.DownloadExampleNote()

	// Then the write error propagates
	assert.Error(t, err)
}
