package ingest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// Progress reports ImportFolder/ImportFile's advance after each file,
// processed or skipped.
type Progress struct {
	FilesProcessed int
	FilesTotal     int
	ChunksCreated  int
	CurrentFile    string
}

// FileFailure records why one file could not be imported.
type FileFailure struct {
	Path   string
	Reason string
}

// Summary is ImportFolder/ImportFile's final report.
type Summary struct {
	FilesScanned  int
	FilesIngested int
	FilesSkipped  int
	FilesFailed   int
	ChunksCreated int
	Failures      []FileFailure
	// IndexWarnings lists files that persisted successfully (ingested_files
	// already recorded their new mtime/model, so a repeat import correctly
	// skips them) but whose in-memory vector index reconciliation failed —
	// distinct from Failures, which are durable write failures.
	IndexWarnings []FileFailure
}

// importCandidate separates the path used to read the current fs.FS from
// the source's canonical identity.
type importCandidate struct {
	// ReadPath is relative to the fs.FS used for this invocation.
	ReadPath string
	// SourcePath is the source's canonical absolute identity, never
	// displayed — combines the desktop-normalized root with ReadPath.
	SourcePath string
}

// ImportFolder walks every .md/.txt file under root (an fs.FS — the
// desktop binding opens the picked directory via os.OpenRoot(path).FS()
// and passes it in, so this use case never touches the os package;
// fstest.MapFS drives it in tests with no temp-dir fixtures), chunking,
// embedding and persisting each one as knowledge_chunks plus a shadow
// knowledge.Item. sourceRoot is the picked directory's canonical absolute
// path (desktop-normalized), combined with each candidate's relative path
// to form its SourcePath identity.
//
// onProgress is called once per file, processed or skipped. If it returns
// a non-nil error, the walk stops immediately and ImportFolder returns the
// Summary accumulated so far alongside that error.
func (s *Service) ImportFolder(
	ctx context.Context, root fs.FS, sourceRoot string, onProgress func(Progress) error,
) (Summary, error) {
	// Held for the entire walk, not just checked once up front — a retry
	// starting mid-import must wait for the whole import to finish rather
	// than interleaving its ListCurrent/ReplaceAll with individual files'
	// transaction commits and VectorStore reconciliation.
	if err := s.index.BeginMutation(); err != nil {
		return Summary{}, err
	}
	defer s.index.EndMutation()

	paths, err := collectCandidates(root)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: scanning folder: %w", err)
	}
	candidates := makeImportCandidates(sourceRoot, paths)
	// restoreDeletedItem is false: folder import preserves 2.3 behavior —
	// an unchanged IngestedFile is skipped without resurrecting a shadow
	// Item deleted in the Explorer.
	return s.importCandidates(ctx, root, candidates, false, onProgress)
}

// ImportFile imports exactly one .md/.txt file, sharing every processing
// step with ImportFolder except restoreDeletedItem (see importCandidates).
// filePath is relative to root; sourceRoot is root's canonical absolute
// path (desktop-normalized).
//
// Input validation deliberately precedes index reservation: an invalid
// request always reports its own error, even while the index is busy.
func (s *Service) ImportFile(
	ctx context.Context, root fs.FS, sourceRoot, filePath string, onProgress func(Progress) error,
) (Summary, error) {
	if !fs.ValidPath(filePath) {
		return Summary{}, fmt.Errorf("ingest: invalid file path %q", filePath)
	}
	switch strings.ToLower(path.Ext(filePath)) {
	case ".md", ".txt":
	default:
		return Summary{}, fmt.Errorf("ingest: unsupported file type %q", path.Ext(filePath))
	}

	if err := s.index.BeginMutation(); err != nil {
		return Summary{}, err
	}
	defer s.index.EndMutation()

	candidate := makeImportCandidate(sourceRoot, filePath)
	// restoreDeletedItem is true: a direct single-file import is an
	// explicit restoration request (see importCandidates).
	return s.importCandidates(ctx, root, []importCandidate{candidate}, true, onProgress)
}

// makeImportCandidate combines sourceRoot with readPath to form one
// importCandidate's canonical SourcePath identity.
func makeImportCandidate(sourceRoot, readPath string) importCandidate {
	return importCandidate{ReadPath: readPath, SourcePath: path.Join(sourceRoot, readPath)}
}

// makeImportCandidates applies makeImportCandidate to every path found by
// collectCandidates.
func makeImportCandidates(sourceRoot string, paths []string) []importCandidate {
	candidates := make([]importCandidate, len(paths))
	for i, p := range paths {
		candidates[i] = makeImportCandidate(sourceRoot, p)
	}
	return candidates
}

// importCandidates is the processing loop shared by ImportFolder and
// ImportFile. Its caller must already hold BeginMutation/EndMutation for
// the whole operation.
//
// restoreDeletedItem is the one intentional processing difference between
// the two entry points:
//   - false (folder import): an unchanged IngestedFile is skipped without
//     resurrecting a shadow Item deleted in the Explorer;
//   - true (single-file import): before skipping an unchanged source, it
//     checks the recorded ItemID. ErrItemNotFound forces the ordinary
//     ingestFile replacement path, which recreates the Item under the same
//     ID and rebuilds its chunks. Another repository error is recorded as
//     that candidate's failure.
func (s *Service) importCandidates(
	ctx context.Context, root fs.FS, candidates []importCandidate, restoreDeletedItem bool,
	onProgress func(Progress) error,
) (Summary, error) {
	existing, err := s.ingestedFiles.ListAll(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: listing previously ingested files: %w", err)
	}

	var summary Summary
	progress := Progress{FilesTotal: len(candidates)}

	for _, candidate := range candidates {
		summary.FilesScanned++
		prev, hasPrev := existing[candidate.SourcePath]
		// The first import's relative path becomes the stable display
		// path; a source already known keeps using the one fixed on its
		// first import, even when reached through another entry point.
		displayPath := candidate.ReadPath
		if hasPrev {
			displayPath = prev.Path
		}
		progress.CurrentFile = displayPath

		mtime, statErr := modTime(root, candidate.ReadPath)
		if statErr != nil {
			summary.FilesFailed++
			summary.Failures = append(summary.Failures, FileFailure{Path: displayPath, Reason: statErr.Error()})
			progress.FilesProcessed++
			if err := onProgress(progress); err != nil {
				return summary, err
			}
			continue
		}

		if hasPrev && prev.MTimeUnixNano == mtime && prev.EmbeddingModel == domainllm.EmbeddingModel {
			skip, failure, checkErr := s.shouldSkipUnchanged(ctx, prev, restoreDeletedItem)
			if checkErr != nil {
				summary.FilesFailed++
				summary.Failures = append(summary.Failures, FileFailure{Path: displayPath, Reason: failure})
				progress.FilesProcessed++
				if err := onProgress(progress); err != nil {
					return summary, err
				}
				continue
			}
			if skip {
				summary.FilesSkipped++
				progress.FilesProcessed++
				if err := onProgress(progress); err != nil {
					return summary, err
				}
				continue
			}
		}

		chunksCreated, ingestErr := s.ingestFile(ctx, root, candidate, displayPath, mtime, prev, hasPrev)
		applyIngestOutcome(&summary, &progress, displayPath, chunksCreated, ingestErr)

		progress.FilesProcessed++
		if err := onProgress(progress); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

// applyIngestOutcome classifies one candidate's ingestFile result and
// updates summary/progress accordingly: a *IndexingWarning (checked first,
// since it is also a non-nil error) counts as ingested with a warning, any
// other non-nil error counts as a failure, and nil counts as a plain
// successful ingest.
func applyIngestOutcome(summary *Summary, progress *Progress, displayPath string, chunksCreated int, ingestErr error) {
	var indexWarning *IndexingWarning
	if errors.As(ingestErr, &indexWarning) {
		// The durable write already succeeded — ingested_files recorded
		// the new mtime/model — so this counts as ingested, not failed.
		summary.FilesIngested++
		summary.ChunksCreated += chunksCreated
		progress.ChunksCreated += chunksCreated
		summary.IndexWarnings = append(summary.IndexWarnings, FileFailure{Path: displayPath, Reason: indexWarning.Error()})
		return
	}
	if ingestErr != nil {
		summary.FilesFailed++
		summary.Failures = append(summary.Failures, FileFailure{Path: displayPath, Reason: ingestErr.Error()})
		return
	}
	summary.FilesIngested++
	summary.ChunksCreated += chunksCreated
	progress.ChunksCreated += chunksCreated
}

// shouldSkipUnchanged decides whether an unchanged source (matching mtime
// and embedding model) should be skipped. Folder import (restoreDeletedItem
// == false) always skips. Single-file import additionally checks that
// prev.ItemID's shadow Item still exists: a deleted Item forces the
// ordinary replacement path instead of skipping, restoring it under the
// same ID; any other repository error is reported as a failure via a
// non-nil returned error, whose message is failure.
func (s *Service) shouldSkipUnchanged(
	ctx context.Context, prev domainknowledge.IngestedFile, restoreDeletedItem bool,
) (skip bool, failure string, err error) {
	if !restoreDeletedItem {
		return true, "", nil
	}
	_, getErr := s.items.GetByID(ctx, prev.ItemID)
	if getErr == nil {
		return true, "", nil
	}
	if errors.Is(getErr, domainknowledge.ErrItemNotFound) {
		return false, "", nil
	}
	return false, getErr.Error(), getErr
}

// modTime returns candidate ReadPath's modification time in root as
// nanoseconds since the Unix epoch — nanosecond precision so a rapid edit
// within the same second is never missed.
func modTime(root fs.FS, readPath string) (int64, error) {
	info, err := fs.Stat(root, readPath)
	if err != nil {
		return 0, err
	}
	return info.ModTime().UnixNano(), nil
}

// collectCandidates pre-walks root for every .md/.txt file (case-
// insensitive extension), skipping any directory whose name starts with
// "." entirely — .git, .obsidian and similar tooling directories never
// contribute candidates.
func collectCandidates(root fs.FS) ([]string, error) {
	var candidates []string
	err := fs.WalkDir(root, ".", func(entryPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if entryPath != "." && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		switch strings.ToLower(path.Ext(entryPath)) {
		case ".md", ".txt":
			candidates = append(candidates, entryPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

// ingestFile reads, chunks, embeds and replaces candidate's stored chunks
// and shadow Item inside a single transaction. Embedding happens before
// anything is deleted, so a failed or interrupted API call leaves the
// previous chunks and Item intact. displayPath is the stable, root-relative
// path used for BuildShadowItem, Chunk.FilePath, and provenance — the
// source's first-import path, even when hasPrev came from another entry
// point's sourceRoot.
func (s *Service) ingestFile(
	ctx context.Context, root fs.FS, candidate importCandidate, displayPath string, mtime int64,
	prev domainknowledge.IngestedFile, hasPrev bool,
) (int, error) {
	raw, err := fs.ReadFile(root, candidate.ReadPath)
	if err != nil {
		return 0, fmt.Errorf("reading file: %w", err)
	}
	content := string(raw)

	var chunkCandidates []ChunkCandidate
	if strings.EqualFold(path.Ext(candidate.ReadPath), ".md") {
		chunkCandidates = ChunkMarkdown(content)
	} else {
		chunkCandidates = ChunkText(content)
	}

	rawTopic, concept, definition := BuildShadowItem(displayPath, content, chunkCandidates)
	topic, err := domainknowledge.NormalizeTopic(rawTopic)
	if err != nil {
		return 0, fmt.Errorf("deriving topic: %w", err)
	}

	itemID := uuid.NewString()
	if hasPrev {
		itemID = prev.ItemID
	}

	now := time.Now().UTC()
	chunks := make([]domainknowledge.Chunk, len(chunkCandidates))
	for i, chunkCandidate := range chunkCandidates {
		response, embedErr := s.llm.Embeddings(ctx, domainllm.EmbeddingRequest{Input: chunkCandidate.Content})
		if embedErr != nil {
			return 0, fmt.Errorf("embedding chunk %d: %w", i, embedErr)
		}
		chunks[i] = domainknowledge.Chunk{
			ID:             uuid.NewString(),
			Source:         domainknowledge.SourceImportedDoc,
			Topic:          topic,
			Status:         domainknowledge.StatusApproved,
			ItemID:         itemID,
			SourcePath:     candidate.SourcePath,
			FilePath:       displayPath,
			Heading:        chunkCandidate.Heading,
			Content:        chunkCandidate.Content,
			Embedding:      toFloat32(response.Embedding),
			EmbeddingModel: domainllm.EmbeddingModel,
			CreatedAt:      now,
		}
	}

	var removedChunkIDs []string
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		removedChunkIDs, err = s.chunks.DeleteBySourcePath(ctx, candidate.SourcePath)
		if err != nil {
			return err
		}
		if err := s.chunks.SaveAll(ctx, chunks); err != nil {
			return err
		}
		if err := s.saveShadowItem(ctx, itemID, topic, concept, definition, now, hasPrev); err != nil {
			return err
		}
		return s.ingestedFiles.Upsert(ctx, domainknowledge.IngestedFile{
			SourcePath:     candidate.SourcePath,
			Path:           displayPath,
			MTimeUnixNano:  mtime,
			EmbeddingModel: domainllm.EmbeddingModel,
			ChunkCount:     len(chunks),
			ItemID:         itemID,
		})
	})
	if err != nil {
		return 0, fmt.Errorf("replacing chunks and item: %w", err)
	}

	// The old memory entries are removed before the replacements are
	// added, so an Add failure can temporarily omit content but can never
	// keep serving stale content.
	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Remove(reconcileCtx, removedChunkIDs); err != nil {
		return len(chunks), &IndexingWarning{Err: err}
	}
	if err := s.store.Add(reconcileCtx, chunks); err != nil {
		return len(chunks), &IndexingWarning{Err: err}
	}
	return len(chunks), nil
}

// saveShadowItem creates itemID on first import, or overwrites its
// Topic/Concept/Definition/UpdatedAt in place on every subsequent one —
// ID, Source, Status and CreatedAt are preserved from the existing record.
//
// hasPrev only means ingested_files still remembers itemID from a past
// import; the Knowledge Explorer's DeleteItem can remove the Item itself
// without touching ingested_files (deleting an imported note's Item must
// not resurrect it on the next unrelated import — see DeleteItem's own
// doc comment). So a hasPrev GetByID miss falls back to recreating the
// Item under the same itemID already baked into this file's chunks,
// instead of failing every subsequent import of that file forever.
func (s *Service) saveShadowItem(
	ctx context.Context, itemID, topic, concept, definition string, now time.Time, hasPrev bool,
) error {
	if hasPrev {
		item, err := s.items.GetByID(ctx, itemID)
		if err == nil {
			item.Topic = topic
			item.Concept = concept
			item.Definition = definition
			item.UpdatedAt = now
			return s.items.Update(ctx, item)
		}
		if !errors.Is(err, domainknowledge.ErrItemNotFound) {
			return err
		}
	}

	return s.items.Save(ctx, domainknowledge.Item{
		ID:         itemID,
		Topic:      topic,
		Concept:    concept,
		Definition: definition,
		Source:     domainknowledge.SourceImportedDoc,
		Status:     domainknowledge.StatusApproved,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

// toFloat32 converts an EmbeddingResponse's float64 vector to the float32
// precision knowledge.Chunk.Embedding stores (see
// internal/infrastructure/sqlite/embedding.go).
func toFloat32(vec []float64) []float32 {
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = float32(v)
	}
	return out
}
