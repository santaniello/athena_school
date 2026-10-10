package ingest

import (
	"context"
	"fmt"
	"io/fs"
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
// approved, so ConfirmImport always imports. onProgress is called exactly
// once, matching ImportFile's one-call-per-file contract.
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

	var summary Summary
	summary.FilesScanned = 1
	progress := Progress{FilesTotal: 1, CurrentFile: displayPath}

	chunksCreated, ingestErr := s.ingestContent(ctx, sessionID, candidate, displayPath, mtime, prev, hasPrev, approvedContent)
	applyIngestOutcome(&summary, &progress, displayPath, chunksCreated, ingestErr)
	progress.FilesProcessed = 1

	if err := onProgress(progress); err != nil {
		return summary, err
	}
	return summary, nil
}
