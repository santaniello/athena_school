package ingest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
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

// statFailFS wraps a MapFS so that Stat on failPath fails, simulating a
// file that vanished between the picker and ImportFile's own stat (e.g. an
// external delete racing the import). fs.Stat prefers a StatFS
// implementation over Open+Stat, so overriding Stat here is enough to
// control modTime's error path.
type statFailFS struct {
	fstest.MapFS
	failPath string
}

func (f statFailFS) Stat(name string) (fs.FileInfo, error) {
	if name == f.failPath {
		return nil, fmt.Errorf("simulated stat failure for %s", name)
	}
	return f.MapFS.Stat(name)
}

var fixedModTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// testSourceRoot is the canonical absolute root every ImportFile test in
// this file imports through, mirroring the desktop-normalized sourceRoot a
// real picked file's parent directory would produce.
const testSourceRoot = "/root"

// srcPath builds the canonical SourcePath a candidate at rel (relative to
// testSourceRoot) resolves to.
func srcPath(rel string) string {
	return path.Join(testSourceRoot, rel)
}

// runWithinTx makes the mocked Transactor behave like the real one: it
// just invokes fn immediately against ctx, so the repo mocks set up
// underneath faithfully observe every call ImportFile makes inside the
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
	store *knowledgemocks.MockVectorStore,
	index IndexGuard,
) *Service {
	return NewService(chunks, ingestedFiles, items, llm, tx, store, index)
}

// passingIndexGuard returns an IndexGuard mock that always allows the
// mutation — the default for every test that isn't specifically about the
// guard rejecting one.
func passingIndexGuard(t *testing.T) *ingestmocks.MockIndexGuard {
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil)
	guard.EXPECT().EndMutation()
	return guard
}

// noOpReconciliationStore returns a VectorStore mock whose Remove/Add both
// succeed as no-ops — the default for tests whose focus is the SQLite/
// orchestration side of ImportFile, not vector-store reconciliation.
func noOpReconciliationStore(t *testing.T) *knowledgemocks.MockVectorStore {
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, mock.Anything).Return(nil)
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)
	return store
}

func embeddingResponse() domainllm.EmbeddingResponse {
	return domainllm.EmbeddingResponse{Embedding: []float64{0.1, 0.2, 0.3}, Model: domainllm.EmbeddingModel}
}

func noopProgress(Progress) error { return nil }

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
	// whose shadow Item was deleted from the Knowledge Explorer — a direct
	// single-file import is an explicit restoration request
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

func TestImportFile_unchangedFile_whenItemLookupFailsForAnotherReason_advancesProgressByOneBeforeFailing(t *testing.T) {
	// Given the same "another repository error" scenario as above, this
	// time observing the progress callback's exact payload
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

	var seen []Progress
	onProgress := func(p Progress) error {
		seen = append(seen, p)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", onProgress)

	// Then progress advances by exactly one for the failed candidate
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	require.Len(t, seen, 1)
	assert.Equal(t, 1, seen[0].FilesProcessed)
}

func TestImportFile_unchangedFile_whenItemLookupFailsForAnotherReason_stopsAndPropagatesOnProgressError(t *testing.T) {
	// Given the same scenario, but this time the progress callback itself
	// signals that the caller wants to stop
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

	stopErr := errors.New("cancelled by caller")
	onProgress := func(Progress) error { return stopErr }
	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", onProgress)

	// Then the callback's error propagates instead of being swallowed by
	// the loop finishing its only candidate normally
	assert.ErrorIs(t, err, stopErr)
	assert.Equal(t, 1, summary.FilesFailed)
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

func TestImportFile_statFailure_isRecordedAsFailure(t *testing.T) {
	// Given a file that can no longer be stat'd once ImportFile reaches it
	// (e.g. an external delete racing the import)
	root := statFailFS{
		MapFS:    fstest.MapFS{"go.md": {Data: []byte("# Go\nBody.")}},
		failPath: "go.md",
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing that single file
	summary, err := service.ImportFile(ctx, root, testSourceRoot, "go.md", noopProgress)

	// Then the unstat-able file is recorded as a failure instead of the
	// request itself erroring
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	require.Len(t, summary.Failures, 1)
	assert.Equal(t, "go.md", summary.Failures[0].Path)
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

	// Then the durable import counts as ingested with a warning
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
