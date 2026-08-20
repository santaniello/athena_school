package desktop

import (
	"log"
	"os"
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/santaniello/athena/internal/application/ingest"
)

// Wails events emitted while a notes import runs. The UI only ever has one
// import active at a time.
const (
	eventIngestProgress = "ingest:progress"
	eventIngestDone     = "ingest:done"
	eventIngestError    = "ingest:error"
)

// IngestProgressResult is the desktop-facing DTO for ImportFolder's
// progress callback.
type IngestProgressResult struct {
	FilesProcessed int    `json:"filesProcessed"`
	FilesTotal     int    `json:"filesTotal"`
	ChunksCreated  int    `json:"chunksCreated"`
	CurrentFile    string `json:"currentFile"`
}

// IngestFailureResult is the desktop-facing DTO for one file that failed
// to import.
type IngestFailureResult struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// IngestSummaryResult is the desktop-facing DTO for ImportFolder's final
// report.
type IngestSummaryResult struct {
	FilesScanned  int                   `json:"filesScanned"`
	FilesIngested int                   `json:"filesIngested"`
	FilesSkipped  int                   `json:"filesSkipped"`
	FilesFailed   int                   `json:"filesFailed"`
	ChunksCreated int                   `json:"chunksCreated"`
	Failures      []IngestFailureResult `json:"failures"`
	// IndexWarnings lists files that persisted successfully — counted in
	// FilesIngested, never in FilesFailed — but whose in-memory vector
	// index reconciliation failed. A full Retry from the Knowledge section
	// self-heals these from SQLite.
	IndexWarnings []IngestFailureResult `json:"indexWarnings"`
}

func toIngestProgressResult(p ingest.Progress) IngestProgressResult {
	return IngestProgressResult{
		FilesProcessed: p.FilesProcessed,
		FilesTotal:     p.FilesTotal,
		ChunksCreated:  p.ChunksCreated,
		CurrentFile:    p.CurrentFile,
	}
}

func toIngestSummaryResult(s ingest.Summary) IngestSummaryResult {
	failures := make([]IngestFailureResult, len(s.Failures))
	for i, f := range s.Failures {
		failures[i] = IngestFailureResult{Path: f.Path, Reason: f.Reason}
	}
	indexWarnings := make([]IngestFailureResult, len(s.IndexWarnings))
	for i, w := range s.IndexWarnings {
		indexWarnings[i] = IngestFailureResult{Path: w.Path, Reason: w.Reason}
		log.Printf("knowledge index: reconciling imported file %q: %s", w.Path, w.Reason)
	}
	return IngestSummaryResult{
		FilesScanned:  s.FilesScanned,
		FilesIngested: s.FilesIngested,
		FilesSkipped:  s.FilesSkipped,
		FilesFailed:   s.FilesFailed,
		ChunksCreated: s.ChunksCreated,
		Failures:      failures,
		IndexWarnings: indexWarnings,
	}
}

// PickNotesFolder opens the OS folder picker and returns the chosen path,
// or "" if the user cancelled.
func (a *App) PickNotesFolder() (string, error) {
	return a.openDirectory(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select notes folder",
	})
}

// PickNotesFile opens the OS file picker restricted to .md/.txt files and
// returns the chosen path, or "" if the user cancelled. The filter exposes
// every casing of .md/.txt because GTK glob matching is case-sensitive
// even though the application rule (see ingest.ImportFile) is not.
func (a *App) PickNotesFile() (string, error) {
	return a.openFile(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select a note file",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Notes (*.md, *.txt)",
				Pattern:     "*.md;*.mD;*.Md;*.MD;*.txt;*.txT;*.tXt;*.tXT;*.Txt;*.TxT;*.TXt;*.TXT",
			},
		},
	})
}

// ImportNotes imports every .md/.txt file under path, streaming progress
// via "ingest:progress" as each file is processed, then emitting
// "ingest:done" with the final summary (or "ingest:error" on failure).
func (a *App) ImportNotes(path string) error {
	root, err := os.OpenRoot(path)
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			log.Printf("closing notes import root %q: %v", path, closeErr)
		}
	}()

	absolutePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}

	summary, err := a.ingest.ImportFolder(a.ctx, root.FS(), filepath.ToSlash(absolutePath), func(p ingest.Progress) error {
		a.emit(a.ctx, eventIngestProgress, toIngestProgressResult(p))
		return nil
	})
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}
	a.emit(a.ctx, eventIngestDone, toIngestSummaryResult(summary))
	return nil
}

// ImportFile imports exactly one .md/.txt file, streaming progress via
// "ingest:progress", then emitting "ingest:done" with the final summary
// (or "ingest:error" on failure). selectedPath is not itself opened as an
// os.Root — only its parent directory is, so the application service still
// never touches OS paths directly.
func (a *App) ImportFile(selectedPath string) error {
	absolutePath, err := filepath.Abs(filepath.Clean(selectedPath))
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}

	dir := filepath.Dir(absolutePath)
	root, err := os.OpenRoot(dir)
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			log.Printf("closing note import root %q: %v", dir, closeErr)
		}
	}()

	summary, err := a.ingest.ImportFile(
		a.ctx,
		root.FS(),
		filepath.ToSlash(dir),
		filepath.Base(absolutePath),
		func(p ingest.Progress) error {
			a.emit(a.ctx, eventIngestProgress, toIngestProgressResult(p))
			return nil
		},
	)
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}
	a.emit(a.ctx, eventIngestDone, toIngestSummaryResult(summary))
	return nil
}
