package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func TestConfirmImport_persistsExactlyTheApprovedContent_notTheFileOnDisk(t *testing.T) {
	// Given a file on disk whose raw content differs from what the user
	// approved in review (the whole point: Confirm trusts the approved
	// text, never re-reads/re-formats the file)
	const onDisk = "messy raw note, never read by ConfirmImport"
	const approved = "# Go\n\nBasics of Go, formatted and approved during review."
	root := fstest.MapFS{"go.md": {Data: []byte(onDisk)}}
	ctx := context.Background()

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Go\n\n" + approved}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].Content == approved && cs[0].FilePath == "go.md"
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Go"
	})).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()

	var seen []Progress
	onProgress := func(p Progress) error {
		seen = append(seen, p)
		return nil
	}
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When confirming the import with the approved (formatted) text
	summary, err := service.ConfirmImport(ctx, testSessionID, root, testSourceRoot, "go.md", approved, onProgress)

	// Then exactly the approved content is persisted — the chunks match
	// what the Review dialog showed, not a re-read of the raw file
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesScanned)
	assert.Equal(t, 1, summary.FilesIngested)
	// One chunk tick (this file has exactly one chunk), then one final
	// file tick — see TestConfirmImport_reportsPerChunkProgress_
	// duringEmbedding for the multi-chunk case in full.
	require.Len(t, seen, 2)
	assert.Equal(t, 1, seen[0].ChunksProcessed)
	assert.Equal(t, 1, seen[0].ChunksTotal)
	assert.Equal(t, 1, seen[1].FilesProcessed)
	assert.Equal(t, 1, seen[1].FilesTotal)
}

func TestConfirmImport_reportsPerChunkProgress_duringEmbedding(t *testing.T) {
	// Given approved content that chunks into 2 sections, each long enough
	// to stand on its own (clears minChunkChars so neither merges into the
	// other)
	section := func(label string) string {
		return strings.Repeat(label+" content long enough to clear the chunk floor on its own. ", 5)
	}
	approved := "## Section A\n\n" + section("Alpha") + "\n\n## Section B\n\n" + section("Beta")
	root := fstest.MapFS{"go.md": {Data: []byte("raw")}}
	ctx := context.Background()

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Twice()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool { return len(cs) == 2 })).
		Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	var seen []Progress
	onProgress := func(p Progress) error {
		seen = append(seen, p)
		return nil
	}

	// When confirming the import
	_, err := service.ConfirmImport(ctx, testSessionID, root, testSourceRoot, "go.md", approved, onProgress)

	// Then onProgress fires once per chunk as it is embedded, reporting
	// intra-file progress, plus one final tick for the finished file
	require.NoError(t, err)
	require.Len(t, seen, 3)
	assert.Equal(t, 1, seen[0].ChunksProcessed)
	assert.Equal(t, 2, seen[0].ChunksTotal)
	assert.Equal(t, 0, seen[0].FilesProcessed, "the file itself isn't done yet on a chunk tick")
	assert.Equal(t, 2, seen[1].ChunksProcessed)
	assert.Equal(t, 2, seen[1].ChunksTotal)
	assert.Equal(t, 1, seen[2].FilesProcessed, "the final tick reports the file, not a chunk")
}

func TestConfirmImport_stopsBeforePersisting_whenAChunkProgressTickFails(t *testing.T) {
	// Given approved content with 2 chunks, and a consumer that rejects
	// the very first progress tick
	section := func(label string) string {
		return strings.Repeat(label+" content long enough to clear the chunk floor on its own. ", 5)
	}
	approved := "## Section A\n\n" + section("Alpha") + "\n\n## Section B\n\n" + section("Beta")
	root := fstest.MapFS{"go.md": {Data: []byte("raw")}}
	ctx := context.Background()

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Once()
	// No EXPECT() at all on chunks/items/tx: a second embedding call, or
	// any persistence call, would fail the test by hitting an unmocked
	// method — proving the abort happens before either.
	chunks := knowledgemocks.NewMockChunkRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	index := ingestmocks.NewMockIndexGuard(t)
	index.EXPECT().BeginMutation().Return(nil)
	index.EXPECT().EndMutation()
	service := &Service{
		chunks: chunks, ingestedFiles: ingestedFiles, items: items, llm: llm, index: index,
	}

	// onProgress fails only on its first call and would succeed on any
	// later one — proving the first tick's own error propagates on its
	// own, not by coincidence of some later call also failing (which
	// previously masked this: the failing tick's error was recorded as an
	// ordinary Summary.Failures entry, and the *next* onProgress call —
	// succeeding here — produced a nil top-level error).
	progressErr := errors.New("progress consumer gone")
	var calls int
	onProgress := func(Progress) error {
		calls++
		if calls == 1 {
			return progressErr
		}
		return nil
	}

	// When confirming the import
	_, err := service.ConfirmImport(ctx, testSessionID, root, testSourceRoot, "go.md", approved, onProgress)

	// Then it stops immediately, before the second chunk is even embedded,
	// and onProgress is never called again after the failing tick
	require.Error(t, err)
	assert.ErrorIs(t, err, progressErr)
	assert.Equal(t, 1, calls)
}

func TestConfirmImport_blankSessionID_isRejectedBeforeReservingTheIndex(t *testing.T) {
	// Given a service whose index guard would fail the test if asked
	service := &Service{index: ingestmocks.NewMockIndexGuard(t)}

	// When confirming with a blank sessionID
	summary, err := service.ConfirmImport(
		context.Background(), "   ", fstest.MapFS{}, testSourceRoot, "go.md", "content", noopProgress,
	)

	// Then it is rejected before the index is ever reserved
	require.Error(t, err)
	assert.ErrorIs(t, err, domainknowledge.ErrSessionRequired)
	assert.Equal(t, Summary{}, summary)
}

func TestConfirmImport_returnsErrIndexLoading_whenIndexIsBusy(t *testing.T) {
	// Given an index guard that reports it is busy
	busyErr := errors.New("index is loading")
	index := ingestmocks.NewMockIndexGuard(t)
	index.EXPECT().BeginMutation().Return(busyErr)
	service := &Service{index: index}

	// When confirming the import
	_, err := service.ConfirmImport(
		context.Background(), testSessionID, fstest.MapFS{}, testSourceRoot, "go.md", "content", noopProgress,
	)

	// Then the busy error is propagated and the index is never released
	// (no matching EndMutation EXPECT — BeginMutation itself failed, so the
	// defer that would call it is never reached)
	require.Error(t, err)
	assert.ErrorIs(t, err, busyErr)
}

func TestConfirmImport_releasesTheIndexReservation_evenOnFailure(t *testing.T) {
	// Given a repository whose ListBySession call fails after the index is
	// reserved
	index := ingestmocks.NewMockIndexGuard(t)
	index.EXPECT().BeginMutation().Return(nil)
	index.EXPECT().EndMutation().Once()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(nil, errors.New("db unavailable")).Once()
	service := &Service{index: index, ingestedFiles: ingestedFiles}

	// When confirming the import
	_, err := service.ConfirmImport(
		context.Background(), testSessionID, fstest.MapFS{"go.md": {Data: []byte("x")}}, testSourceRoot, "go.md",
		"content", noopProgress,
	)

	// Then the error propagates and EndMutation still ran (asserted via the
	// mock's Once() expectation above)
	require.Error(t, err)
}

func TestConfirmImport_stopsAndPropagatesOnProgressError(t *testing.T) {
	// Given a file that ingests successfully
	const approved = "# Go\n\nBasics of Go, long enough to stand as its own chunk."
	root := fstest.MapFS{"go.md": {Data: []byte("raw")}}
	ctx := context.Background()

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Go\n\n" + approved}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// Fails only the final (file-level) tick, not the chunk tick that
	// precedes it — this test is specifically about a failure *after*
	// persistence already succeeded; a chunk-tick failure (which aborts
	// *before* persistence) is TestConfirmImport_
	// stopsBeforePersisting_whenAChunkProgressTickFails's job.
	progressErr := errors.New("progress consumer gone")
	var calls int
	onProgress := func(Progress) error {
		calls++
		if calls == 2 {
			return progressErr
		}
		return nil
	}

	// When the onProgress callback fails on its final call
	summary, err := service.ConfirmImport(ctx, testSessionID, root, testSourceRoot, "go.md", approved, onProgress)

	// Then the already-accumulated summary is still returned, alongside
	// the propagated error — same contract ImportFile's loop has
	require.Error(t, err)
	assert.ErrorIs(t, err, progressErr)
	assert.Equal(t, 1, summary.FilesIngested)
}
