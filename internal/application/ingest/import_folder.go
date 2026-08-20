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

// Progress reports ImportFolder's advance after each file, processed or
// skipped.
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

// Summary is ImportFolder's final report.
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

// ImportFolder walks every .md/.txt file under root (an fs.FS — the
// desktop binding opens the picked directory via os.OpenRoot(path).FS()
// and passes it in, so this use case never touches the os package;
// fstest.MapFS drives it in tests with no temp-dir fixtures), chunking,
// embedding and persisting each one as knowledge_chunks plus a shadow
// knowledge.Item.
//
// onProgress is called once per file, processed or skipped. If it returns
// a non-nil error, the walk stops immediately and ImportFolder returns the
// Summary accumulated so far alongside that error.
func (s *Service) ImportFolder(ctx context.Context, root fs.FS, onProgress func(Progress) error) (Summary, error) {
	// Held for the entire walk, not just checked once up front — a retry
	// starting mid-import must wait for the whole import to finish rather
	// than interleaving its ListCurrent/ReplaceAll with individual files'
	// transaction commits and VectorStore reconciliation.
	if err := s.index.BeginMutation(); err != nil {
		return Summary{}, err
	}
	defer s.index.EndMutation()

	candidates, err := collectCandidates(root)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: scanning folder: %w", err)
	}

	existing, err := s.ingestedFiles.ListAll(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: listing previously ingested files: %w", err)
	}

	var summary Summary
	progress := Progress{FilesTotal: len(candidates)}

	for _, filePath := range candidates {
		summary.FilesScanned++
		progress.CurrentFile = filePath

		mtime, statErr := modTime(root, filePath)
		if statErr != nil {
			summary.FilesFailed++
			summary.Failures = append(summary.Failures, FileFailure{Path: filePath, Reason: statErr.Error()})
			progress.FilesProcessed++
			if err := onProgress(progress); err != nil {
				return summary, err
			}
			continue
		}

		prev, hasPrev := existing[filePath]
		if hasPrev && prev.MTime == mtime && prev.EmbeddingModel == domainllm.EmbeddingModel {
			summary.FilesSkipped++
			progress.FilesProcessed++
			if err := onProgress(progress); err != nil {
				return summary, err
			}
			continue
		}

		chunksCreated, ingestErr := s.ingestFile(ctx, root, filePath, mtime, prev, hasPrev)
		var indexWarning *IndexingWarning
		switch {
		case errors.As(ingestErr, &indexWarning):
			// The durable write already succeeded — ingested_files recorded
			// the new mtime/model — so this counts as ingested, not failed.
			summary.FilesIngested++
			summary.ChunksCreated += chunksCreated
			progress.ChunksCreated += chunksCreated
			summary.IndexWarnings = append(summary.IndexWarnings, FileFailure{Path: filePath, Reason: indexWarning.Error()})
		case ingestErr == nil:
			summary.FilesIngested++
			summary.ChunksCreated += chunksCreated
			progress.ChunksCreated += chunksCreated
		default:
			summary.FilesFailed++
			summary.Failures = append(summary.Failures, FileFailure{Path: filePath, Reason: ingestErr.Error()})
		}

		progress.FilesProcessed++
		if err := onProgress(progress); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

// modTime returns path's modification time as Unix seconds.
func modTime(root fs.FS, filePath string) (int64, error) {
	info, err := fs.Stat(root, filePath)
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
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

// ingestFile reads, chunks, embeds and replaces filePath's stored chunks
// and shadow Item inside a single transaction. Embedding happens before
// anything is deleted, so a failed or interrupted API call leaves the
// previous chunks and Item intact.
func (s *Service) ingestFile(
	ctx context.Context, root fs.FS, filePath string, mtime int64,
	prev domainknowledge.IngestedFile, hasPrev bool,
) (int, error) {
	raw, err := fs.ReadFile(root, filePath)
	if err != nil {
		return 0, fmt.Errorf("reading file: %w", err)
	}
	content := string(raw)

	var candidates []ChunkCandidate
	if strings.EqualFold(path.Ext(filePath), ".md") {
		candidates = ChunkMarkdown(content)
	} else {
		candidates = ChunkText(content)
	}

	rawTopic, concept, definition := BuildShadowItem(filePath, content, candidates)
	topic, err := domainknowledge.NormalizeTopic(rawTopic)
	if err != nil {
		return 0, fmt.Errorf("deriving topic: %w", err)
	}

	itemID := uuid.NewString()
	if hasPrev {
		itemID = prev.ItemID
	}

	now := time.Now().UTC()
	chunks := make([]domainknowledge.Chunk, len(candidates))
	for i, candidate := range candidates {
		response, embedErr := s.llm.Embeddings(ctx, domainllm.EmbeddingRequest{Input: candidate.Content})
		if embedErr != nil {
			return 0, fmt.Errorf("embedding chunk %d: %w", i, embedErr)
		}
		chunks[i] = domainknowledge.Chunk{
			ID:             uuid.NewString(),
			Source:         domainknowledge.SourceImportedDoc,
			Topic:          topic,
			Status:         domainknowledge.StatusApproved,
			ItemID:         itemID,
			FilePath:       filePath,
			Heading:        candidate.Heading,
			Content:        candidate.Content,
			Embedding:      toFloat32(response.Embedding),
			EmbeddingModel: domainllm.EmbeddingModel,
			CreatedAt:      now,
		}
	}

	var removedChunkIDs []string
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		removedChunkIDs, err = s.chunks.DeleteByFilePath(ctx, filePath)
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
			Path:           filePath,
			MTime:          mtime,
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
