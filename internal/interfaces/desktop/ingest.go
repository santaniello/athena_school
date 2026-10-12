package desktop

import (
	"log"
	"os"
	"path/filepath"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/santaniello/athena/internal/application/ingest"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// Wails events emitted while a notes import runs. The UI only ever has one
// import active at a time.
const (
	eventIngestProgress           = "ingest:progress"
	eventIngestDone               = "ingest:done"
	eventIngestError              = "ingest:error"
	eventIngestFormattingProgress = "ingest:formatting-progress"
)

// IngestProgressResult is the desktop-facing DTO for ImportFile's/
// ConfirmImport's/PreviewImport's progress callback. ChunksProcessed/
// ChunksTotal are 0 for every tick ImportFile's own multi-file batch
// reports — only ConfirmImport (spec 2.20) ever sets them, as each chunk
// of the single file it is confirming gets embedded. SectionsProcessed/
// SectionsTotal are 0 everywhere except PreviewImport (spec 2.20.1), as each
// heading-bounded batch of the file being previewed gets formatted.
type IngestProgressResult struct {
	FilesProcessed    int    `json:"filesProcessed"`
	FilesTotal        int    `json:"filesTotal"`
	ChunksCreated     int    `json:"chunksCreated"`
	CurrentFile       string `json:"currentFile"`
	ChunksProcessed   int    `json:"chunksProcessed"`
	ChunksTotal       int    `json:"chunksTotal"`
	SectionsProcessed int    `json:"sectionsProcessed"`
	SectionsTotal     int    `json:"sectionsTotal"`
}

// IngestFailureResult is the desktop-facing DTO for one file that failed
// to import.
type IngestFailureResult struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// IngestSummaryResult is the desktop-facing DTO for ImportFile's final
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
		FilesProcessed:    p.FilesProcessed,
		FilesTotal:        p.FilesTotal,
		ChunksCreated:     p.ChunksCreated,
		CurrentFile:       p.CurrentFile,
		ChunksProcessed:   p.ChunksProcessed,
		ChunksTotal:       p.ChunksTotal,
		SectionsProcessed: p.SectionsProcessed,
		SectionsTotal:     p.SectionsTotal,
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

// ImportFile imports exactly one .md/.txt file into the study session
// sessionID, streaming progress via
// "ingest:progress", then emitting "ingest:done" with the final summary
// (or "ingest:error" on failure). selectedPath is not itself opened as an
// os.Root — only its parent directory is, so the application service still
// never touches OS paths directly.
func (a *App) ImportFile(sessionID, selectedPath string) error {
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
		sessionID,
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

// ChunkPreviewResult is the desktop-facing DTO for one would-be chunk shown
// in the Review dialog before import is approved. See
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
type ChunkPreviewResult struct {
	Heading         string `json:"heading"`
	Content         string `json:"content"`
	CharCount       int    `json:"charCount"`
	EstimatedTokens int    `json:"estimatedTokens"`
}

// FormattingStatsResult is the desktop-facing DTO for PreviewImport's
// before/after structural counts.
type FormattingStatsResult struct {
	HeadingsBefore   int `json:"headingsBefore"`
	HeadingsAfter    int `json:"headingsAfter"`
	ParagraphsBefore int `json:"paragraphsBefore"`
	ParagraphsAfter  int `json:"paragraphsAfter"`
}

// FailedSectionResult is the desktop-facing DTO for one formatting batch
// that fell back to its own original content.
type FailedSectionResult struct {
	Heading string `json:"heading"`
	Reason  string `json:"reason"`
}

// StructureWarningResult is the desktop-facing DTO for one accepted batch
// whose chunking outcome still looks unhealthy (see ingest.StructureWarning).
type StructureWarningResult struct {
	Heading string `json:"heading"`
	Message string `json:"message"`
}

// ImportPreviewResult is the desktop-facing DTO for PreviewImport's result.
// Skipped is true when the file is unchanged since its last import — the
// Review dialog is not opened for it at all.
type ImportPreviewResult struct {
	DisplayPath string `json:"displayPath"`
	// SourceURL is the canonical URL of a link preview; empty for a file.
	SourceURL         string                   `json:"sourceUrl"`
	OriginalContent   string                   `json:"originalContent"`
	FormattedContent  string                   `json:"formattedContent"`
	FormattingFailed  bool                     `json:"formattingFailed"`
	FailedSections    []FailedSectionResult    `json:"failedSections"`
	StructureWarnings []StructureWarningResult `json:"structureWarnings"`
	Stats             FormattingStatsResult    `json:"stats"`
	Chunks            []ChunkPreviewResult     `json:"chunks"`
	Skipped           bool                     `json:"skipped"`
	SkippedReason     string                   `json:"skippedReason"`
}

func toImportPreviewResult(p ingest.ImportPreview) ImportPreviewResult {
	chunks := make([]ChunkPreviewResult, len(p.Chunks))
	for i, c := range p.Chunks {
		chunks[i] = ChunkPreviewResult{
			Heading: c.Heading, Content: c.Content, CharCount: c.CharCount, EstimatedTokens: c.EstimatedTokens,
		}
	}
	failedSections := make([]FailedSectionResult, len(p.FailedSections))
	for i, f := range p.FailedSections {
		failedSections[i] = FailedSectionResult{Heading: f.Heading, Reason: f.Reason}
	}
	structureWarnings := make([]StructureWarningResult, len(p.StructureWarnings))
	for i, w := range p.StructureWarnings {
		structureWarnings[i] = StructureWarningResult{Heading: w.Heading, Message: w.Message}
	}
	return ImportPreviewResult{
		DisplayPath:       p.DisplayPath,
		SourceURL:         p.SourceURL,
		OriginalContent:   p.OriginalContent,
		FormattedContent:  p.FormattedContent,
		FormattingFailed:  p.FormattingFailed,
		FailedSections:    failedSections,
		StructureWarnings: structureWarnings,
		Stats: FormattingStatsResult{
			HeadingsBefore:   p.Stats.HeadingsBefore,
			HeadingsAfter:    p.Stats.HeadingsAfter,
			ParagraphsBefore: p.Stats.ParagraphsBefore,
			ParagraphsAfter:  p.Stats.ParagraphsAfter,
		},
		Chunks:        chunks,
		Skipped:       p.Skipped,
		SkippedReason: p.SkippedReason,
	}
}

// PreviewImport reformats and chunks selectedPath without persisting
// anything — no embeddings, no database write. It opens the same os.Root
// around the file's parent directory ImportFile does, but is a plain
// request/response call: unlike ImportFile/ConfirmImport, it never emits
// "ingest:progress"/"ingest:done"/"ingest:error", since nothing it does is
// a durable operation those track. It does emit
// "ingest:formatting-progress" while formatting is in flight (spec 2.20.1),
// a distinct event FormattingNoteDialog alone subscribes to.
func (a *App) PreviewImport(sessionID, selectedPath string) (ImportPreviewResult, error) {
	absolutePath, err := filepath.Abs(filepath.Clean(selectedPath))
	if err != nil {
		return ImportPreviewResult{}, err
	}

	dir := filepath.Dir(absolutePath)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return ImportPreviewResult{}, err
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			log.Printf("closing note preview root %q: %v", dir, closeErr)
		}
	}()

	preview, err := a.ingest.PreviewImport(
		a.ctx, sessionID, root.FS(), filepath.ToSlash(dir), filepath.Base(absolutePath),
		func(p ingest.Progress) error {
			a.emit(a.ctx, eventIngestFormattingProgress, toIngestProgressResult(p))
			return nil
		},
	)
	if err != nil {
		return ImportPreviewResult{}, err
	}
	return toImportPreviewResult(preview), nil
}

// ConfirmImport persists exactly approvedContent — the text a prior
// PreviewImport call returned and the user approved in the Review dialog
// — as selectedPath's chunks and shadow Item, streaming progress via the
// same "ingest:progress"/"ingest:done"/"ingest:error" events ImportFile
// emits, so IngestProgressDialog needs no new event wiring.
func (a *App) ConfirmImport(sessionID, selectedPath, approvedContent string) error {
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
			log.Printf("closing note confirm root %q: %v", dir, closeErr)
		}
	}()

	summary, err := a.ingest.ConfirmImport(
		a.ctx, sessionID, root.FS(), filepath.ToSlash(dir), filepath.Base(absolutePath), approvedContent,
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

// PreviewLinkImport fetches rawURL and reformats and chunks the page
// without persisting anything — the link counterpart of PreviewImport,
// emitting the same "ingest:formatting-progress" ticks while formatting is
// in flight. Its result's SourceURL and DisplayPath are what the frontend
// echoes back to ConfirmLinkImport.
func (a *App) PreviewLinkImport(sessionID, rawURL string) (ImportPreviewResult, error) {
	preview, err := a.ingest.PreviewURL(a.ctx, sessionID, rawURL, func(p ingest.Progress) error {
		a.emit(a.ctx, eventIngestFormattingProgress, toIngestProgressResult(p))
		return nil
	})
	if err != nil {
		return ImportPreviewResult{}, err
	}
	return toImportPreviewResult(preview), nil
}

// ConfirmLinkImport persists exactly approvedContent — the text a prior
// PreviewLinkImport returned and the user approved — as rawURL's source,
// streaming progress via the same "ingest:progress"/"ingest:done"/
// "ingest:error" events ConfirmImport emits.
func (a *App) ConfirmLinkImport(sessionID, rawURL, title, approvedContent string) error {
	summary, err := a.ingest.ConfirmURLImport(
		a.ctx, sessionID, rawURL, title, approvedContent,
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

// SessionSourceResult is the desktop-facing DTO for one document a session
// has imported, listed in the Sources panel. See
// specs/phases/phase-02-knowledge-engine/17-session-sources-panel.md.
type SessionSourceResult struct {
	ItemID     string `json:"itemId"`
	Title      string `json:"title"`
	Path       string `json:"path"`
	ChunkCount int    `json:"chunkCount"`
	IngestedAt string `json:"ingestedAt"`
}

func toSessionSourceResult(s domainknowledge.SessionSource) SessionSourceResult {
	return SessionSourceResult{
		ItemID:     s.ItemID,
		Title:      s.Title,
		Path:       s.Path,
		ChunkCount: s.ChunkCount,
		IngestedAt: s.IngestedAt.Format(time.RFC3339),
	}
}

// ListSessionSources returns sessionID's imported documents for the
// Sources panel, oldest-imported first.
func (a *App) ListSessionSources(sessionID string) ([]SessionSourceResult, error) {
	sources, err := a.ingest.ListSources(a.ctx, sessionID)
	if err != nil {
		return nil, err
	}
	results := make([]SessionSourceResult, len(sources))
	for i, s := range sources {
		results[i] = toSessionSourceResult(s)
	}
	return results, nil
}

// RemoveSessionSource hard-deletes itemID's chunks, its knowledge Item and
// its ingested_files record from sessionID — never the file on disk, and
// never another session's copy of it.
func (a *App) RemoveSessionSource(sessionID, itemID string) error {
	return a.ingest.RemoveSource(a.ctx, sessionID, itemID)
}

// SourceDocumentSegmentResult is the desktop-facing DTO for one ordered
// slice of a source document's text: either a chunk's own passage
// (ChunkID set) or the gap between two chunks (ChunkID empty).
type SourceDocumentSegmentResult struct {
	Text    string `json:"text"`
	ChunkID string `json:"chunkId"`
}

// SourceDocumentResult is the desktop-facing DTO for a session's document,
// as read by the source viewer.
type SourceDocumentResult struct {
	ItemID   string                        `json:"itemId"`
	Title    string                        `json:"title"`
	Path     string                        `json:"path"`
	Segments []SourceDocumentSegmentResult `json:"segments"`
}

func toSourceDocumentResult(doc domainknowledge.SourceDocument) SourceDocumentResult {
	segments := make([]SourceDocumentSegmentResult, len(doc.Segments))
	for i, s := range doc.Segments {
		segments[i] = SourceDocumentSegmentResult{Text: s.Text, ChunkID: s.ChunkID}
	}
	return SourceDocumentResult{
		ItemID:   doc.ItemID,
		Title:    doc.Title,
		Path:     doc.Path,
		Segments: segments,
	}
}

// GetSessionSourceDocument returns itemID's full document for the source
// viewer, scoped to sessionID. Errors propagate as-is: ingest.ErrSourceNotFound
// (itemID does not exist, or belongs to another session) and
// ingest.ErrSourceTextUnavailable (no document text is stored — a document
// imported before spec 2.18, or whose text was otherwise lost).
func (a *App) GetSessionSourceDocument(sessionID, itemID string) (SourceDocumentResult, error) {
	doc, err := a.ingest.GetSourceDocument(a.ctx, sessionID, itemID)
	if err != nil {
		return SourceDocumentResult{}, err
	}
	return toSourceDocumentResult(doc), nil
}
