package ingest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

var fixedModTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// runWithinTx makes the mocked Transactor behave like the real one: it
// just invokes fn immediately against ctx, so the repo mocks set up
// underneath faithfully observe every call ImportFolder makes inside the
// transactional replace step.
func runWithinTx(tx *ingestmocks.MockTransactor) {
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

func newTestService(
	chunks *knowledgemocks.MockChunkRepository,
	ingestedFiles *knowledgemocks.MockIngestedFileRepository,
	items *knowledgemocks.MockRepository,
	llm *llmmocks.MockProvider,
	tx *ingestmocks.MockTransactor,
) *Service {
	return NewService(chunks, ingestedFiles, items, llm, tx)
}

func embeddingResponse() domainllm.EmbeddingResponse {
	return domainllm.EmbeddingResponse{Embedding: []float64{0.1, 0.2, 0.3}, Model: domainllm.EmbeddingModel}
}

func noopProgress(Progress) error { return nil }

func TestImportFolder_ingestsMarkdownAndTxtFiles_skipsHiddenDirectoriesEntirely(t *testing.T) {
	// Given a folder with one markdown file, one text file, and a hidden
	// directory that would otherwise contribute a matching file
	root := fstest.MapFS{
		"notes/go.md":            {Data: []byte("# Go\nBasics of Go.")},
		"notes/plain.txt":        {Data: []byte("Plain text note without any heading.")},
		".obsidian/workspace.md": {Data: []byte("# Should never be seen\nbody")},
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nBasics of Go."}).
		Return(embeddingResponse(), nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Plain text note without any heading."}).
		Return(embeddingResponse(), nil).Once()

	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/plain.txt").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].FilePath == "notes/go.md" && cs[0].Heading == "Go"
	})).Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].FilePath == "notes/plain.txt" && cs[0].Heading == ""
	})).Return(nil).Once()

	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Go" && item.Source == domainknowledge.SourceImportedDoc && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "plain" && item.Source == domainknowledge.SourceImportedDoc
	})).Return(nil).Once()

	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "notes/go.md" && f.ChunkCount == 1
	})).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "notes/plain.txt" && f.ChunkCount == 1
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then only the two real files are scanned/ingested; the hidden
	// directory never contributes a candidate
	require.NoError(t, err)
	assert.Equal(t, 2, summary.FilesScanned)
	assert.Equal(t, 2, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesSkipped)
	assert.Equal(t, 0, summary.FilesFailed)
	assert.Equal(t, 2, summary.ChunksCreated)
}

func TestImportFolder_reimport_isIdempotent_whenNothingChanged(t *testing.T) {
	// Given one file already recorded with its current mtime and embedding
	// model (alphabetically first, so it is skipped before the walk
	// reaches the second, brand-new file) and one file never seen before
	root := fstest.MapFS{
		"notes/a-unchanged.md": {Data: []byte("# A\nUnchanged."), ModTime: fixedModTime},
		"notes/b-new.md":       {Data: []byte("# B\nBrand new.")},
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		"notes/a-unchanged.md": {
			Path: "notes/a-unchanged.md", MTime: fixedModTime.Unix(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# B\nBrand new."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/b-new.md").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	var seen []Progress
	onProgress := func(p Progress) error {
		seen = append(seen, p)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, onProgress)

	// Then the unchanged file is skipped (no repository writes or LLM
	// calls for it) while the walk still continues on to ingest the new
	// one, and FilesProcessed advances by exactly one per file, in order
	require.NoError(t, err)
	assert.Equal(t, 2, summary.FilesScanned)
	assert.Equal(t, 1, summary.FilesSkipped)
	assert.Equal(t, 1, summary.FilesIngested)
	require.Len(t, seen, 2)
	assert.Equal(t, 1, seen[0].FilesProcessed)
	assert.Equal(t, 2, seen[1].FilesProcessed)
}

func TestImportFolder_editedFile_reembedsOnlyThatFile_andUpdatesExistingShadowItemInPlace(t *testing.T) {
	// Given a file whose recorded mtime differs from its current one (edited)
	root := fstest.MapFS{"notes/go.md": {Data: []byte("# Go\nUpdated body."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		"notes/go.md": {
			Path: "notes/go.md", MTime: fixedModTime.Add(-time.Hour).Unix(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()

	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nUpdated body."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()

	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "notes", Concept: "Go", Definition: "old",
		Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	items.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Definition == "Updated body." &&
			item.Source == domainknowledge.SourceImportedDoc && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()

	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "notes/go.md" && f.ItemID == "item-1"
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When re-importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then only that file is reprocessed, and items.Save is never called —
	// the existing Item is updated in place, not duplicated
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesSkipped)
	items.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestImportFolder_reimport_afterItemDeleted_recreatesTheShadowItemInsteadOfFailingForever(t *testing.T) {
	// Given a file whose ingested_files record still points at item-1, but
	// the Knowledge Explorer has since deleted that Item (DeleteItem never
	// touches ingested_files — see its own doc comment)
	root := fstest.MapFS{"notes/go.md": {Data: []byte("# Go\nUpdated body."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		"notes/go.md": {
			Path: "notes/go.md", MTime: fixedModTime.Add(-time.Hour).Unix(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()

	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nUpdated body."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()

	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Definition == "Updated body." &&
			item.Source == domainknowledge.SourceImportedDoc && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()

	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "notes/go.md" && f.ItemID == "item-1"
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When re-importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the file is ingested successfully — a fresh Item is recreated
	// under the same ID already baked into its chunks, rather than the
	// import permanently failing on ErrItemNotFound — and Update is never
	// called, since there is nothing to update
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesFailed)
	items.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestImportFolder_perFileFailure_isRecordedInSummary_andImportContinues(t *testing.T) {
	// Given two files, one of which fails to embed
	root := fstest.MapFS{
		"notes/bad.md":  {Data: []byte("# Bad\nWill fail to embed.")},
		"notes/good.md": {Data: []byte("# Good\nWill embed fine.")},
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	boom := errors.New("embedding provider unavailable")
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Bad\nWill fail to embed."}).
		Return(domainllm.EmbeddingResponse{}, boom).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Good\nWill embed fine."}).
		Return(embeddingResponse(), nil).Once()

	chunks.EXPECT().DeleteByFilePath(ctx, "notes/good.md").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the failure is recorded and the other file still imports
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	assert.Equal(t, 1, summary.FilesIngested)
	require.Len(t, summary.Failures, 1)
	assert.Equal(t, "notes/bad.md", summary.Failures[0].Path)
	assert.Contains(t, summary.Failures[0].Reason, boom.Error())
}

func TestImportFolder_onProgressError_stopsWalkImmediately_returningPartialSummary(t *testing.T) {
	// Given two files that would both otherwise import successfully
	root := fstest.MapFS{
		"notes/a.md": {Data: []byte("# A\nBody A.")},
		"notes/b.md": {Data: []byte("# B\nBody B.")},
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# A\nBody A."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/a.md").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	stopErr := errors.New("cancelled by caller")
	calls := 0
	onProgress := func(Progress) error {
		calls++
		return stopErr
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When the progress callback fails after the first file
	summary, err := service.ImportFolder(ctx, root, onProgress)

	// Then the walk stops immediately: only "notes/a.md" was processed, and
	// "notes/b.md" (alphabetically after) never reaches the LLM or repos
	assert.ErrorIs(t, err, stopErr)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 1, summary.FilesScanned)
	assert.Equal(t, 1, summary.FilesIngested)
}

func TestImportFolder_changedEmbeddingModel_forcesReembed_evenWhenMTimeIsUnchanged(t *testing.T) {
	// Given a file recorded with the same mtime but a different embedding model
	root := fstest.MapFS{"notes/go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		"notes/go.md": {
			Path: "notes/go.md", MTime: fixedModTime.Unix(),
			EmbeddingModel: "some-old-model", ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()

	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nBasics of Go."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].EmbeddingModel == domainllm.EmbeddingModel
	})).Return(nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	items.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.EmbeddingModel == domainllm.EmbeddingModel
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When re-importing despite the unchanged mtime
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then it is re-embedded rather than skipped
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesSkipped)
}

func TestImportFolder_manyFiles_completesReportingProgressPerFile(t *testing.T) {
	// Given a folder with 100 markdown files
	const fileCount = 100
	root := fstest.MapFS{}
	for i := 0; i < fileCount; i++ {
		root[fmt.Sprintf("notes/note-%03d.md", i)] = &fstest.MapFile{
			Data: []byte(fmt.Sprintf("# Note %d\nBody for note %d.", i, i)),
		}
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Times(fileCount)
	chunks.EXPECT().DeleteByFilePath(ctx, mock.Anything).Return(nil).Times(fileCount)
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Times(fileCount)
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Times(fileCount)
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Times(fileCount)

	progressCalls := 0
	onProgress := func(p Progress) error {
		progressCalls++
		assert.Equal(t, fileCount, p.FilesTotal)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx)

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, onProgress)

	// Then every file is processed and reported without crashing
	require.NoError(t, err)
	assert.Equal(t, fileCount, summary.FilesScanned)
	assert.Equal(t, fileCount, summary.FilesIngested)
	assert.Equal(t, fileCount, progressCalls)
}
