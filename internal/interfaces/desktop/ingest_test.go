package desktop

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	applicationingest "github.com/santaniello/athena/internal/application/ingest"
	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

// testIngestSessionID is the study session every ImportFile binding test
// imports into.
const testIngestSessionID = "session-1"

// normalizedSourceRoot mirrors the desktop adapter's own normalization
// (filepath.Abs + filepath.Clean + filepath.ToSlash) so tests can predict
// the exact SourcePath a given real directory resolves to.
func normalizedSourceRoot(t *testing.T, dir string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Clean(dir))
	require.NoError(t, err)
	return filepath.ToSlash(abs)
}

// capturedIngestEvents records every ingest:* event emitted through
// App.emit during a test.
type capturedIngestEvents struct {
	progress []IngestProgressResult
	done     *IngestSummaryResult
	errors   []string
}

func newTestIngestApp(
	t *testing.T,
	chunks domainknowledge.ChunkRepository,
	ingestedFiles domainknowledge.IngestedFileRepository,
	items domainknowledge.Repository,
	documents domainknowledge.DocumentRepository,
	llm domainllm.Provider,
	store domainknowledge.VectorStore,
) (*App, *capturedIngestEvents) {
	t.Helper()
	tx := ingestmocks.NewMockTransactor(t)
	// app.Startup(context.Background()) below stashes that exact context on
	// a.ctx, and every dependency call forwards it unchanged — matching it
	// precisely (instead of mock.Anything) means this helper would fail if
	// ImportFile ever stopped propagating it.
	tx.EXPECT().WithinTx(context.Background(), mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).Maybe()
	guard := ingestmocks.NewMockIndexGuard(t)
	// .Maybe(): a path that fails before ever reaching ImportFile (e.g. a
	// file that doesn't exist) never calls the guard at all.
	guard.EXPECT().BeginMutation().Return(nil).Maybe()
	guard.EXPECT().EndMutation().Maybe()
	ingestService := applicationingest.NewService(chunks, ingestedFiles, items, documents, llm, tx, store, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(context.Background())

	captured := &capturedIngestEvents{}
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		switch eventName {
		case eventIngestProgress:
			p := data[0].(IngestProgressResult)
			captured.progress = append(captured.progress, p)
		case eventIngestDone:
			s := data[0].(IngestSummaryResult)
			captured.done = &s
		case eventIngestError:
			captured.errors = append(captured.errors, data[0].(string))
		}
	}
	return app, captured
}

func TestApp_PickNotesFile_usesTheExactTitleAndCaseCompleteFilter(t *testing.T) {
	// Given an App whose file-picker dialog captures the options it was called with
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	var captured wailsruntime.OpenDialogOptions
	app.openFile = func(_ context.Context, options wailsruntime.OpenDialogOptions) (string, error) {
		captured = options
		return "/home/user/notes/go.md", nil
	}

	// When picking a notes file
	path, err := app.PickNotesFile()

	// Then the chosen path is returned, and the dialog was configured with
	// the exact title and every casing of .md/.txt (GTK glob matching is
	// case-sensitive even though the application rule is not)
	require.NoError(t, err)
	assert.Equal(t, "/home/user/notes/go.md", path)
	assert.Equal(t, "Select a note file", captured.Title)
	require.Len(t, captured.Filters, 1)
	assert.Equal(t, "Notes (*.md, *.txt)", captured.Filters[0].DisplayName)
	assert.Equal(t, "*.md;*.mD;*.Md;*.MD;*.txt;*.txT;*.tXt;*.tXT;*.Txt;*.TxT;*.TXt;*.TXT", captured.Filters[0].Pattern)
}

func TestApp_PickNotesFile_returnsError_whenDialogFails(t *testing.T) {
	// Given a file-picker dialog that fails
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	boom := errors.New("dialog unavailable")
	app.openFile = func(context.Context, wailsruntime.OpenDialogOptions) (string, error) {
		return "", boom
	}

	// When picking a notes file
	_, err := app.PickNotesFile()

	// Then the error propagates
	assert.ErrorIs(t, err, boom)
}

func TestApp_PickNotesFile_returnsEmptyPath_onCancellation(t *testing.T) {
	// Given a file-picker dialog the user cancelled
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	app.openFile = func(context.Context, wailsruntime.OpenDialogOptions) (string, error) {
		return "", nil
	}

	// When picking a notes file
	path, err := app.PickNotesFile()

	// Then no error occurs and the empty path passes through as-is
	require.NoError(t, err)
	assert.Empty(t, path)
}

func TestApp_ImportFile_emitsProgressThenDone_onSuccess(t *testing.T) {
	// Given a real single markdown file, and a fully-mocked knowledge/LLM
	// stack that accepts it
	dir := t.TempDir()
	filePath := filepath.Join(dir, "go.md")
	require.NoError(t, os.WriteFile(filePath, []byte("# Go\nBasics of Go."), 0o600))

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)

	ctx := context.Background()
	sourcePath := path.Join(normalizedSourceRoot(t, dir), "go.md")
	ingestedFiles.EXPECT().ListBySession(ctx, testIngestSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nBasics of Go."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}, Model: domainllm.EmbeddingModel}, nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testIngestSessionID, sourcePath).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].FilePath == "go.md" && cs[0].SourcePath == sourcePath
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "go.md" && f.SourcePath == sourcePath
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, ([]string)(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Save(ctx, mock.Anything, testIngestSessionID, "# Go\nBasics of Go.").Return(nil).Once()

	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, documents, llm, store)

	// When importing that file through the desktop adapter
	err := app.ImportFile(testIngestSessionID, filePath)

	// Then progress is emitted for the one file, followed by a done
	// summary reporting it ingested — 1 of 1 files
	require.NoError(t, err)
	require.Len(t, captured.progress, 1)
	assert.Equal(t, 1, captured.progress[0].FilesTotal)
	assert.Equal(t, "go.md", captured.progress[0].CurrentFile)
	require.NotNil(t, captured.done)
	assert.Equal(t, 1, captured.done.FilesIngested)
	assert.Empty(t, captured.errors)
}

func TestApp_ImportFile_emitsError_whenFileDoesNotExist(t *testing.T) {
	// Given a path that does not exist on disk
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, nil, llm, nil)

	// When importing it
	err := app.ImportFile(testIngestSessionID, filepath.Join(t.TempDir(), "does-not-exist", "go.md"))

	// Then an "ingest:error" event is emitted and the error is returned,
	// with no "ingest:done" ever firing
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Nil(t, captured.done)
}

func TestApp_ImportFile_emitsError_whenImportFileFails(t *testing.T) {
	// Given a file whose only candidate fails to list previously-ingested
	// state (simulating a repository failure)
	dir := t.TempDir()
	filePath := filepath.Join(dir, "go.md")
	require.NoError(t, os.WriteFile(filePath, []byte("# Go\nBody."), 0o600))
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	boom := errors.New("database unavailable")
	ingestedFiles.EXPECT().ListBySession(context.Background(), testIngestSessionID).Return(nil, boom).Once()
	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, nil, llm, nil)

	// When importing that file
	err := app.ImportFile(testIngestSessionID, filePath)

	// Then the failure surfaces as an "ingest:error" event
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Contains(t, captured.errors[0], boom.Error())
	assert.Nil(t, captured.done)
}

func TestApp_ImportFile_emitsError_whenExtensionIsUnsupported(t *testing.T) {
	// Given a real file whose extension is not .md/.txt
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notes.pdf")
	require.NoError(t, os.WriteFile(filePath, []byte("not a note"), 0o600))
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, nil, llm, nil)

	// When importing it
	err := app.ImportFile(testIngestSessionID, filePath)

	// Then it is rejected before ever reaching the index/repositories
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Nil(t, captured.done)
	ingestedFiles.AssertNotCalled(t, "ListBySession", mock.Anything, mock.Anything)
}

func TestApp_ListSessionSources_returnsTheMappedResults(t *testing.T) {
	// Given a session with one imported document
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	ingestedFiles.EXPECT().ListSourcesBySession(ctx, testIngestSessionID).Return([]domainknowledge.SessionSource{
		{ItemID: "item-1", Title: "CAP theorem", Path: "cap.md", ChunkCount: 4, IngestedAt: ingestedAt},
	}, nil).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	ingestService := applicationingest.NewService(nil, ingestedFiles, nil, nil, nil, nil, nil, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When listing the session's sources
	results, err := app.ListSessionSources(testIngestSessionID)

	// Then it returns the DTO, with IngestedAt formatted as RFC3339
	require.NoError(t, err)
	assert.Equal(t, []SessionSourceResult{
		{ItemID: "item-1", Title: "CAP theorem", Path: "cap.md", ChunkCount: 4, IngestedAt: "2024-01-02T03:04:05Z"},
	}, results)
}

func TestApp_ListSessionSources_returnsTheServiceError(t *testing.T) {
	// Given a service that fails to list
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	boom := errors.New("database unavailable")
	ingestedFiles.EXPECT().ListSourcesBySession(ctx, testIngestSessionID).Return(nil, boom).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	ingestService := applicationingest.NewService(nil, ingestedFiles, nil, nil, nil, nil, nil, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When listing the session's sources
	_, err := app.ListSessionSources(testIngestSessionID)

	// Then the error propagates
	assert.ErrorIs(t, err, boom)
}

func TestApp_RemoveSessionSource_delegatesToTheIngestService(t *testing.T) {
	// Given a document owned by the session
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testIngestSessionID}, nil).Once()
	items.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().DeleteByItemID(ctx, testIngestSessionID, "item-1").Return(nil).Once()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().DeleteByItemID(ctx, testIngestSessionID, "item-1").Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(nil).Once()
	tx := ingestmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	ingestService := applicationingest.NewService(chunks, ingestedFiles, items, documents, nil, tx, store, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When removing it
	err := app.RemoveSessionSource(testIngestSessionID, "item-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_RemoveSessionSource_returnsErrSourceNotFound_forAnotherSessionsDocument(t *testing.T) {
	// Given a document owned by a different session
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: "other-session"}, nil).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(nil).Once()
	guard.EXPECT().EndMutation().Once()
	ingestService := applicationingest.NewService(nil, nil, items, nil, nil, nil, nil, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When testIngestSessionID tries to remove it
	err := app.RemoveSessionSource(testIngestSessionID, "item-1")

	// Then it is rejected, so one session can never delete another's document
	assert.ErrorIs(t, err, applicationingest.ErrSourceNotFound)
}

func TestApp_GetSessionSourceDocument_returnsTheMappedDocument(t *testing.T) {
	// Given a document owned by the session
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{ID: "item-1", SessionID: testIngestSessionID, Concept: "CAP theorem"}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(ctx, testIngestSessionID, "item-1").Return("hello world", nil).Once()
	start, end := 0, 5
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListByItemID(ctx, "item-1").Return([]domainknowledge.Chunk{
		{ID: "chunk-1", FilePath: "cap.md", StartOffset: &start, EndOffset: &end},
	}, nil).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	ingestService := applicationingest.NewService(chunks, nil, items, documents, nil, nil, nil, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When getting its document
	result, err := app.GetSessionSourceDocument(testIngestSessionID, "item-1")

	// Then it returns the mapped DTO, segments in order
	require.NoError(t, err)
	assert.Equal(t, SourceDocumentResult{
		ItemID: "item-1", Title: "CAP theorem", Path: "cap.md",
		Segments: []SourceDocumentSegmentResult{
			{Text: "hello", ChunkID: "chunk-1"},
			{Text: " world"},
		},
	}, result)
}

func TestApp_GetSessionSourceDocument_returnsErrSourceNotFound_forAnotherSessionsDocument(t *testing.T) {
	// Given a document owned by a different session
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: "other-session"}, nil).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	ingestService := applicationingest.NewService(nil, nil, items, nil, nil, nil, nil, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When testIngestSessionID tries to get its document
	_, err := app.GetSessionSourceDocument(testIngestSessionID, "item-1")

	// Then it is rejected, so one session can never read another's document
	assert.ErrorIs(t, err, applicationingest.ErrSourceNotFound)
}

func TestApp_GetSessionSourceDocument_returnsErrSourceTextUnavailable_whenNoTextIsStored(t *testing.T) {
	// Given an item imported before document text was stored
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testIngestSessionID}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(ctx, testIngestSessionID, "item-1").Return("", domainknowledge.ErrDocumentNotFound).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	ingestService := applicationingest.NewService(nil, nil, items, documents, nil, nil, nil, guard)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)

	// When getting its document
	_, err := app.GetSessionSourceDocument(testIngestSessionID, "item-1")

	// Then it reports that the text is unavailable rather than opening
	assert.ErrorIs(t, err, applicationingest.ErrSourceTextUnavailable)
}
