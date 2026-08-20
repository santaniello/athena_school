package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	llm domainllm.Provider,
) (*App, *capturedIngestEvents) {
	t.Helper()
	tx := ingestmocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).Maybe()
	ingestService := applicationingest.NewService(chunks, ingestedFiles, items, llm, tx)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, ingestService, nil)
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

func TestApp_ImportNotes_emitsProgressThenDone_onSuccess(t *testing.T) {
	// Given a real folder with one markdown file, and a fully-mocked
	// knowledge/LLM stack that accepts it
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.md"), []byte("# Go\nBasics of Go."), 0o600))

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)

	// App.Startup(context.Background()) stashes that exact context on
	// a.ctx, and ImportNotes forwards it unchanged into every dependency
	// call below — matching it precisely (instead of mock.Anything) means
	// this test would fail if ImportNotes ever stopped propagating it.
	ctx := context.Background()
	ingestedFiles.EXPECT().ListAll(ctx).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "# Go\nBasics of Go."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}, Model: domainllm.EmbeddingModel}, nil).Once()
	chunks.EXPECT().DeleteByFilePath(ctx, "go.md").Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].FilePath == "go.md" && cs[0].Heading == "Go" &&
			cs[0].Source == domainknowledge.SourceImportedDoc && cs[0].Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "Go" && item.Definition == "Basics of Go." &&
			item.Source == domainknowledge.SourceImportedDoc && item.Status == domainknowledge.StatusApproved
	})).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == "go.md" && f.ChunkCount == 1 && f.EmbeddingModel == domainllm.EmbeddingModel
	})).Return(nil).Once()

	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, llm)

	// When importing that folder through the desktop adapter
	err := app.ImportNotes(dir)

	// Then progress is emitted for the one file, followed by a done
	// summary reporting it ingested
	require.NoError(t, err)
	require.Len(t, captured.progress, 1)
	assert.Equal(t, 1, captured.progress[0].FilesTotal)
	assert.Equal(t, "go.md", captured.progress[0].CurrentFile)
	require.NotNil(t, captured.done)
	assert.Equal(t, 1, captured.done.FilesIngested)
	assert.Empty(t, captured.errors)
}

func TestApp_ImportNotes_emitsError_whenFolderDoesNotExist(t *testing.T) {
	// Given a path that does not exist on disk
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, llm)

	// When importing it
	err := app.ImportNotes(filepath.Join(t.TempDir(), "does-not-exist"))

	// Then an "ingest:error" event is emitted and the error is returned,
	// with no "ingest:done" ever firing
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Nil(t, captured.done)
}

func TestApp_ImportNotes_emitsError_whenImportFolderFails(t *testing.T) {
	// Given a folder whose only file fails to list previously-ingested
	// state (simulating a repository failure)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.md"), []byte("# Go\nBody."), 0o600))
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	boom := errors.New("database unavailable")
	ingestedFiles.EXPECT().ListAll(mock.Anything).Return(nil, boom).Once()
	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, llm)

	// When importing that folder
	err := app.ImportNotes(dir)

	// Then the failure surfaces as an "ingest:error" event
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Contains(t, captured.errors[0], boom.Error())
	assert.Nil(t, captured.done)
}

func TestApp_PickNotesFolder_returnsThePathChosenByTheDialog(t *testing.T) {
	// Given an App whose folder-picker dialog is stubbed to return a path
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	app.openDirectory = func(context.Context, wailsruntime.OpenDialogOptions) (string, error) {
		return "/home/user/notes", nil
	}

	// When picking a notes folder
	path, err := app.PickNotesFolder()

	// Then the chosen path is returned as-is
	require.NoError(t, err)
	assert.Equal(t, "/home/user/notes", path)
}

func TestApp_PickNotesFolder_returnsError_whenDialogFails(t *testing.T) {
	// Given a folder-picker dialog that fails
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.Startup(context.Background())
	boom := errors.New("dialog unavailable")
	app.openDirectory = func(context.Context, wailsruntime.OpenDialogOptions) (string, error) {
		return "", boom
	}

	// When picking a notes folder
	_, err := app.PickNotesFolder()

	// Then the error propagates
	assert.ErrorIs(t, err, boom)
}
