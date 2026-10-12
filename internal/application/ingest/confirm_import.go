package ingest

import (
	"context"
	"fmt"
	"io/fs"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// ConfirmImport persists exactly approvedContent — the text a
// PreviewImport call already formatted, chunked and the user approved — as
// candidate's chunks and shadow Item. It never re-reads or re-formats the
// file: approvedContent is what gets chunked and embedded, so the chunks
// the user reviewed are the chunks that get persisted (spec 2.20 decision
// 4). Unlike PreviewImport, this reserves the index for its call's
// duration, exactly as ImportFile does — this is the step that actually
// writes.
//
// There is no second skip-unchanged check here: the caller already went
// through PreviewImport (which does check) and the user explicitly
// approved, so ConfirmImport always imports. onProgress is called once per
// chunk as it is embedded (ChunksProcessed/ChunksTotal set, FilesProcessed
// still 0 — the file itself isn't done yet), then exactly once more for the
// finished file (FilesProcessed: 1), matching ImportFile's one-call-per-
// file contract for that final tick. A non-nil error from any call stops
// the import immediately, before anything is persisted, and propagates
// directly as this call's own error — never downgraded into an ordinary
// per-file Summary.Failures entry the way an embedding/chunking failure
// is, since a failing progress callback means the caller itself can no
// longer be reached, not that this file failed to import.
func (s *Service) ConfirmImport(
	ctx context.Context, sessionID string, root fs.FS, sourceRoot, filePath, approvedContent string,
	onProgress func(Progress) error,
) (Summary, error) {
	if err := validateImportTarget(sessionID, filePath); err != nil {
		return Summary{}, err
	}

	if err := s.index.BeginMutation(); err != nil {
		return Summary{}, err
	}
	defer s.index.EndMutation()

	candidate := makeImportCandidate(sourceRoot, filePath)

	existing, err := s.ingestedFiles.ListBySession(ctx, sessionID)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: listing previously ingested files: %w", err)
	}
	prev, hasPrev := existing[candidate.SourcePath]
	displayPath := filePath
	if hasPrev {
		displayPath = prev.Path
	}

	mtime, err := modTime(root, candidate.ReadPath)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: reading file metadata: %w", err)
	}

	return s.persistApproved(ctx, sessionID, candidate, displayPath, mtime, prev, hasPrev, approvedContent, onProgress)
}

// persistApproved embeds and stores approvedContent as candidate's chunks and
// shadow Item, reporting progress per chunk and once for the finished source.
// It is the write step shared by ConfirmImport (a file) and ConfirmURLImport
// (a fetched page); the caller has already reserved the index.
func (s *Service) persistApproved(
	ctx context.Context, sessionID string, candidate importCandidate, displayPath string, mtime int64,
	prev domainknowledge.IngestedFile, hasPrev bool, approvedContent string, onProgress func(Progress) error,
) (Summary, error) {
	var summary Summary
	summary.FilesScanned = 1
	base := Progress{FilesTotal: 1, CurrentFile: displayPath}

	// progressErr captures a failure from onProgress itself (e.g. the
	// consumer on the other side of the IPC channel is gone) as distinct
	// from an ingestErr caused by embedding/chunking/persistence — the
	// former must propagate directly, never get recorded as an ordinary
	// per-file failure in summary via applyIngestOutcome, which would let
	// a later, successful onProgress call mask it into a nil-error return.
	var progressErr error
	onChunkEmbedded := func(processed, total int) error {
		tick := base
		tick.ChunksProcessed = processed
		tick.ChunksTotal = total
		if err := onProgress(tick); err != nil {
			progressErr = err
			return err
		}
		return nil
	}

	chunksCreated, ingestErr := s.ingestContent(
		ctx, sessionID, candidate, displayPath, mtime, prev, hasPrev, approvedContent, onChunkEmbedded,
	)
	if progressErr != nil {
		return Summary{}, progressErr
	}

	progress := base
	applyIngestOutcome(&summary, &progress, displayPath, chunksCreated, ingestErr)
	progress.FilesProcessed = 1

	if err := onProgress(progress); err != nil {
		return summary, err
	}
	return summary, nil
}
