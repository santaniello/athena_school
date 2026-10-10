package desktop

import (
	"os"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// exampleNoteMarkdown demonstrates the note structure the chunking
// pipeline (see internal/application/ingest/chunking.go) works best with:
// each sub-topic inside a heading section gets its own paragraph,
// separated by a blank line, instead of several ideas bundled into one
// continuous paragraph — the latter produces one diluted chunk covering
// all of them, which chunking-by-paragraph cannot split on its own. See
// specs/phases/phase-02-knowledge-engine/19-example-note-template.md.
const exampleNoteMarkdown = `# Example Note — How Athena Reads This File

> Delete this note after reading — it's a template, not real content.
> Athena splits a note into chunks by paragraph inside each heading
> section. A heading that covers more than one idea works best when each
> idea gets its own paragraph (separated by a blank line), like the
> section below — instead of one long paragraph mixing them together.

### HTTP Methods

GET requests a resource without changing anything on the server, and is
safe to retry or cache.

POST creates a new resource or triggers a side effect, and is not safe to
retry blindly — retrying a POST can create the same resource twice.

PUT replaces a resource entirely at a known URL, and is idempotent:
sending the same PUT twice has the same effect as sending it once.

DELETE removes a resource at a known URL, and is also idempotent — deleting
an already-deleted resource is still just "gone" either way.

### Why One Paragraph Per Idea

Each paragraph above talks about exactly one HTTP method. If this whole
section were written as a single paragraph mixing GET, POST, PUT and
DELETE together, a question about just one of them — "what does PUT do?"
— would have to compete with the other three ideas crammed into the same
piece of text, making it harder to answer precisely from this note alone.
`

// DownloadExampleNote opens the OS save dialog, pre-filled with
// "example-note.md", then writes the bundled example note (see
// exampleNoteMarkdown) to the chosen path. Returns "" if the user
// cancelled — mirrors PickNotesFile's own cancellation contract.
func (a *App) DownloadExampleNote() (string, error) {
	path, err := a.saveFile(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save example note",
		DefaultFilename: "example-note.md",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Markdown (*.md)", Pattern: "*.md"},
		},
	})
	if err != nil || path == "" {
		return path, err
	}
	if err := os.WriteFile(path, []byte(exampleNoteMarkdown), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
