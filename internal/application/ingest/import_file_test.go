package ingest

import (
	"context"
	"errors"
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

func TestImportFile_newFile_ingestsExactlyOneFileWithFilesTotalOne(t *testing.T) {
	// Given one never-before-seen markdown file
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go.")}}
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
	chunks.EXPECT().DeleteBySourcePath(ctx, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].FilePath == "go.md" && cs[0].SourcePath == srcPath("go.md")
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Go" && item.Source == domainknowledge.SourceImportedDoc
	})).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "go.md" && f.SourcePath == srcPath("go.md")
	})).Return(nil).Once()

	var seen []Progress
	onProgress := func(p Progress) error {
		seen = append(seen, p)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", onProgress)

	// Then it is ingested and reported as a one-file operation
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesScanned)
	assert.Equal(t, 1, summary.FilesIngested)
	require.Len(t, seen, 1)
	assert.Equal(t, 1, seen[0].FilesTotal)
	assert.Equal(t, "go.md", seen[0].CurrentFile)
}

func TestImportFile_unchangedFile_withExistingItem_isSkipped(t *testing.T) {
	// Given a file already recorded with its current mtime/model, whose
	// shadow Item still exists
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.UnixNano(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then it is skipped — no embedding, transacting, or saving
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesSkipped)
	assert.Equal(t, 0, summary.FilesIngested)
	llm.AssertNotCalled(t, "Embeddings", mock.Anything, mock.Anything)
}

func TestImportFile_unchangedFile_withDeletedItem_restoresItUnderTheSameID(t *testing.T) {
	// Given a file already recorded with its current mtime/model, but
	// whose shadow Item was deleted from the Knowledge Explorer — direct
	// single-file import is an explicit restoration request, unlike folder
	// import which would just skip this
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.UnixNano(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Twice()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nBasics of Go."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1"
	})).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.ItemID == "item-1"
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then the Item is rebuilt under the same recorded ID and counted as
	// ingested, not skipped
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesSkipped)
}

func TestImportFolder_unchangedFile_withDeletedItem_stillSkips(t *testing.T) {
	// Given the same scenario reached through folder import instead —
	// restoreDeletedItem is false there, so it continues to skip
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.UnixNano(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing the folder containing that file
	summary, err := service.ImportFolder(ctx, root, testSourceRoot, noopProgress)

	// Then it is skipped without ever checking the Item's existence
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesSkipped)
	items.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestImportFile_unchangedFile_whenItemLookupFailsForAnotherReason_isRecordedAsFailure(t *testing.T) {
	// Given a file that would otherwise be skipped, but whose restoration
	// check hits a genuine repository error (not ErrItemNotFound)
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.UnixNano(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	boom := errors.New("database unavailable")
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{}, boom).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then the candidate is recorded as failed, not silently skipped or
	// force-replaced
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	assert.Equal(t, 0, summary.FilesSkipped)
	require.Len(t, summary.Failures, 1)
	assert.Contains(t, summary.Failures[0].Reason, boom.Error())
}

func TestImportFile_changedFile_reembedsAndUpdatesInPlace(t *testing.T) {
	// Given a file whose recorded mtime differs from its current one
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nUpdated body."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.Add(-time.Hour).UnixNano(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nUpdated body."}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	items.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then it is re-embedded and the existing Item updated in place
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	items.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestImportFile_caseInsensitiveExtension_isAccepted(t *testing.T) {
	// Given a file whose extension casing differs from the canonical form
	root := fstest.MapFS{"GO.MD": {Data: []byte("# Go\nBasics of Go.")}}
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
	chunks.EXPECT().DeleteBySourcePath(ctx, srcPath("GO.MD")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "GO.MD", noopProgress)

	// Then it is accepted and ingested despite the uppercase extension
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
}

func TestImportFile_rejectsInvalidPath_beforeReservingTheIndex(t *testing.T) {
	// Given a path that is not a valid fs.FS path
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	guard := ingestmocks.NewMockIndexGuard(t) // no expectations: never called

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, guard)

	// When importing an invalid path
	summary, err := service.ImportFile(ctx, fstest.MapFS{}, testSourceRoot, "../escape.md", noopProgress)

	// Then it is rejected as a top-level error before ever touching the index
	require.Error(t, err)
	assert.Equal(t, Summary{}, summary)
}

func TestImportFile_rejectsUnsupportedExtension_beforeReservingTheIndex(t *testing.T) {
	// Given a file whose extension is neither .md nor .txt
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	guard := ingestmocks.NewMockIndexGuard(t) // no expectations: never called

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, guard)

	// When importing an unsupported file type
	summary, err := service.ImportFile(ctx, fstest.MapFS{"notes.pdf": {}}, testSourceRoot, "notes.pdf", noopProgress)

	// Then it is rejected as a top-level error before ever touching the index
	require.Error(t, err)
	assert.Equal(t, Summary{}, summary)
}

func TestImportFile_returnsErrIndexLoading_whenIndexIsBusy(t *testing.T) {
	// Given a loading/retrying index
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	boom := errors.New("knowledge index is loading")
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(boom).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, guard)

	// When importing a valid file while the index is busy
	summary, err := service.ImportFile(ctx, fstest.MapFS{"go.md": {}}, testSourceRoot, "go.md", noopProgress)

	// Then the reservation rejection propagates and nothing else runs
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, Summary{}, summary)
	ingestedFiles.AssertNotCalled(t, "ListAll", mock.Anything)
}

func TestImportFile_perFileFailure_isRecordedInSummary_notAsATopLevelError(t *testing.T) {
	// Given a valid candidate whose embedding call fails
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go.")}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	boom := errors.New("embedding provider unavailable")
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nBasics of Go."}).
		Return(domainllm.EmbeddingResponse{}, boom).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then the request-level call still succeeds, with the failure inside the Summary
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	require.Len(t, summary.Failures, 1)
	assert.Contains(t, summary.Failures[0].Reason, boom.Error())
}

func TestImportFile_reportsIndexingWarning_butStillCountsTheFileAsIngested(t *testing.T) {
	// Given a file whose durable import succeeds but whose store
	// reconciliation fails
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go.")}}
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
	chunks.EXPECT().DeleteBySourcePath(ctx, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, mock.Anything).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(boom).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, store, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then the durable import counts as ingested with a warning, matching
	// ImportFolder's IndexWarnings behavior
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesFailed)
	require.Len(t, summary.IndexWarnings, 1)
}

func TestImportFile_fullReservationRelease_allowsASubsequentImport(t *testing.T) {
	// Given a service whose IndexGuard tracks Begin/End calls precisely
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go.")}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Twice()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Twice()
	chunks.EXPECT().DeleteBySourcePath(ctx, mock.Anything).Return(nil, nil).Twice()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Twice()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Twice()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Twice()

	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Twice()
	guard.EXPECT().EndMutation().Twice()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), guard)

	// When importing the same file twice in a row
	_, err1 := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)
	_, err2 := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then each call fully released the reservation for the next one
	require.NoError(t, err1)
	require.NoError(t, err2)
}
