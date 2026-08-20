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
	guard.EXPECT().CheckMutationAllowed().Return(nil)
	return guard
}

// noOpReconciliationStore returns a VectorStore mock whose Remove/Add both
// succeed as no-ops — the default for tests whose focus is the SQLite/
// orchestration side of ImportFolder, not vector-store reconciliation.
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

	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil, nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/plain.txt").Return(nil, nil).Once()
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

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

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

func TestImportFolder_emptyFile_stillGetsExactlyOneItem_withZeroChunks(t *testing.T) {
	// Given a completely empty .md file
	root := fstest.MapFS{"notes/empty.md": {Data: []byte("")}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/empty.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 0
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Definition == emptyFileDefinition
	})).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "notes/empty.md" && f.ChunkCount == 0
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the file is ingested as a single Item with zero chunks (and no
	// LLM call, since there is nothing to embed), rather than failing —
	// the shadow Item's placeholder Definition keeps it valid to save
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesFailed)
	assert.Equal(t, 0, summary.ChunksCreated)
	llm.AssertNotCalled(t, "Embeddings", mock.Anything, mock.Anything)
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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/b-new.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	var seen []Progress
	onProgress := func(p Progress) error {
		seen = append(seen, p)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil, nil).Once()
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

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When re-importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then only that file is reprocessed, and items.Save is never called —
	// the existing Item is updated in place, not duplicated
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesSkipped)
	items.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestImportFolder_editedFile_evictsOldChunkIDsFromTheStore_beforeAddingTheNewOnes(t *testing.T) {
	// Given a file whose SQLite-deleted chunk IDs are known, so the memory
	// eviction can be observed precisely — the old memory entries must be
	// removed before the replacements are added, so an Add failure can
	// temporarily omit content but can never keep serving stale content
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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return([]string{"old-chunk-1"}, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "notes", Concept: "Go", Definition: "old",
		Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	items.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	var order []string
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"old-chunk-1"}).Run(func(context.Context, []string) {
		order = append(order, "remove")
	}).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Run(func(context.Context, []domainknowledge.Chunk) {
		order = append(order, "add")
	}).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, store, passingIndexGuard(t))

	// When re-importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the SQLite-deleted ID is evicted from the store before the new
	// chunk is added, and the durable import succeeds
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Empty(t, summary.IndexWarnings)
	assert.Equal(t, []string{"remove", "add"}, order)
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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil, nil).Once()
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

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

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

	chunks.EXPECT().DeleteByFilePath(ctx, "notes/good.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

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

func TestImportFolder_perFileFailure_whenDerivedTopicIsBlank(t *testing.T) {
	// Given a root-level file named exactly ".md" (no directory to derive a
	// topic from, and no heading): path.Ext(".md") == ".md", so
	// baseNameWithoutExt strips the entire name down to "", and
	// BuildShadowItem's fallback chain would otherwise produce an empty Topic
	root := fstest.MapFS{
		".md": {Data: []byte("Some body text with no heading.")},
	}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	// No WithinTx expectation: the blank topic is caught before the file
	// ever reaches the transactional replace step, so it must never be called.
	tx := ingestmocks.NewMockTransactor(t)

	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	// No VectorStore expectation either: the blank topic is caught before
	// ingestFile ever reaches the post-commit reconciliation step.
	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, passingIndexGuard(t))

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the file is recorded as failed instead of persisting a
	// blank-topic chunk/item, and nothing is embedded, transacted, or saved
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	require.Len(t, summary.Failures, 1)
	assert.Equal(t, ".md", summary.Failures[0].Path)
	assert.Contains(t, summary.Failures[0].Reason, domainknowledge.ErrTopicRequired.Error())
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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/a.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	stopErr := errors.New("cancelled by caller")
	calls := 0
	onProgress := func(Progress) error {
		calls++
		return stopErr
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].EmbeddingModel == domainllm.EmbeddingModel
	})).Return(nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	items.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.EmbeddingModel == domainllm.EmbeddingModel
	})).Return(nil).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

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
	chunks.EXPECT().DeleteByFilePath(ctx, mock.Anything).Return(nil, nil).Times(fileCount)
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Times(fileCount)
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Times(fileCount)
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Times(fileCount)

	progressCalls := 0
	onProgress := func(p Progress) error {
		progressCalls++
		assert.Equal(t, fileCount, p.FilesTotal)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, onProgress)

	// Then every file is processed and reported without crashing
	require.NoError(t, err)
	assert.Equal(t, fileCount, summary.FilesScanned)
	assert.Equal(t, fileCount, summary.FilesIngested)
	assert.Equal(t, fileCount, progressCalls)
}

func TestImportFolder_returnsErrIndexLoading_whenIndexIsLoading_andNeverScansTheFolder(t *testing.T) {
	// Given a loading/retrying index
	root := fstest.MapFS{"notes/go.md": {Data: []byte("# Go\nBasics of Go.")}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	boom := errors.New("knowledge index is loading")
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(boom).Once()
	service := newTestService(chunks, ingestedFiles, items, llm, tx, nil, guard)

	// When importing the folder
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the import is rejected before ever listing ingested files —
	// a retry snapshot can never be overwritten by a concurrent import
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, Summary{}, summary)
	ingestedFiles.AssertNotCalled(t, "ListAll", mock.Anything)
}

func TestImportFolder_reportsIndexingWarning_whenPostCommitReconciliationFails_butStillCountsTheFileAsIngested(t *testing.T) {
	// Given a file whose durable import succeeds
	root := fstest.MapFS{"notes/go.md": {Data: []byte("# Go\nBasics of Go.")}}
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
	chunks.EXPECT().DeleteByFilePath(ctx, "notes/go.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	boom := errors.New("store exploded")
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, mock.Anything).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(boom).Once()

	service := newTestService(chunks, ingestedFiles, items, llm, tx, store, passingIndexGuard(t))

	// When importing the folder and the post-commit reconciliation fails
	summary, err := service.ImportFolder(ctx, root, noopProgress)

	// Then the durable import is not reported as a failure — ingested_files
	// already recorded the new mtime/model, so a repeat import would
	// legitimately skip it — the file is counted as ingested with a warning
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesFailed)
	assert.Empty(t, summary.Failures)
	require.Len(t, summary.IndexWarnings, 1)
	assert.Equal(t, "notes/go.md", summary.IndexWarnings[0].Path)
	assert.Contains(t, summary.IndexWarnings[0].Reason, boom.Error())
}
