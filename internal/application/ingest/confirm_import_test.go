package ingest

import (
	"context"
	"errors"
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
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: approved}).
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
	require.Len(t, seen, 1)
	assert.Equal(t, 1, seen[0].FilesProcessed)
	assert.Equal(t, 1, seen[0].FilesTotal)
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
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: approved}).
		Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	progressErr := errors.New("progress consumer gone")
	onProgress := func(Progress) error { return progressErr }

	// When the onProgress callback itself fails
	summary, err := service.ConfirmImport(ctx, testSessionID, root, testSourceRoot, "go.md", approved, onProgress)

	// Then the already-accumulated summary is still returned, alongside
	// the propagated error — same contract ImportFile's loop has
	require.Error(t, err)
	assert.ErrorIs(t, err, progressErr)
	assert.Equal(t, 1, summary.FilesIngested)
}
