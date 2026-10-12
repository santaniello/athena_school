package desktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationingest "github.com/santaniello/athena/internal/application/ingest"
	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	"github.com/santaniello/athena/internal/domain/web"
	webmocks "github.com/santaniello/athena/internal/domain/web/mocks"
)

const testLinkURL = "https://example.com/article"

func TestApp_PreviewLinkImport_returnsTheMappedPreview(t *testing.T) {
	// Given a fetchable article and a provider that formats it
	ctx := context.Background()
	fetcher := webmocks.NewMockFetcher(t)
	fetcher.EXPECT().Fetch(ctx, testLinkURL).
		Return(web.Page{URL: testLinkURL, Title: "Go Article", Markdown: "raw article"}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(ctx, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.Task == domainllm.TaskNoteFormatting
	})).Return(domainllm.ChatResponse{Content: "# Go Article\n\nFormatted article, long enough to stand as its own chunk on its own."}, nil).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(nil).Once()
	ingestService := applicationingest.NewService(nil, nil, nil, nil, llm, nil, nil, guard, fetcher)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(ctx)
	var formattingProgress []IngestProgressResult
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName == eventIngestFormattingProgress {
			formattingProgress = append(formattingProgress, data[0].(IngestProgressResult))
		}
	}

	// When previewing the link through the desktop adapter
	result, err := app.PreviewLinkImport(testIngestSessionID, testLinkURL)

	// Then it returns the mapped preview with the canonical URL and title,
	// having emitted one formatting-progress tick
	require.NoError(t, err)
	assert.Equal(t, testLinkURL, result.SourceURL)
	assert.Equal(t, "Go Article", result.DisplayPath)
	assert.Equal(t, "# Go Article\n\nraw article", result.OriginalContent)
	assert.Contains(t, result.FormattedContent, "Formatted article")
	require.Len(t, result.Chunks, 1)
	require.Len(t, formattingProgress, 1)
	assert.Equal(t, 1, formattingProgress[0].SectionsTotal)
}

func TestApp_PreviewLinkImport_returnsTheServiceError(t *testing.T) {
	// Given a fetcher that refuses the host
	fetcher := webmocks.NewMockFetcher(t)
	fetcher.EXPECT().Fetch(mock.Anything, testLinkURL).Return(web.Page{}, web.ErrBlockedHost).Once()
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(nil).Once()
	ingestService := applicationingest.NewService(nil, nil, nil, nil, nil, nil, nil, guard, fetcher)
	app := NewApp(nil, nil, nil, nil, nil, nil, ingestService, nil, nil, nil)
	app.Startup(context.Background())

	// When previewing the link
	_, err := app.PreviewLinkImport(testIngestSessionID, testLinkURL)

	// Then the typed error propagates to the frontend
	assert.ErrorIs(t, err, web.ErrBlockedHost)
}

func TestApp_ConfirmLinkImport_emitsProgressThenDone_onSuccess(t *testing.T) {
	// Given approved (already-formatted) text for a link
	const approved = "# Go\nBasics of Go, approved during review."
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	ingestedFiles.EXPECT().ListBySession(ctx, testIngestSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Go\n\n" + approved}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}, Model: domainllm.EmbeddingModel}, nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testIngestSessionID, testLinkURL).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].Content == approved && cs[0].SourcePath == testLinkURL
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.Anything).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, ([]string)(nil)).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Save(ctx, mock.Anything, testIngestSessionID, approved).Return(nil).Once()
	app, captured := newTestIngestApp(t, chunks, ingestedFiles, items, documents, llm, store)

	// When confirming the link import through the desktop adapter
	err := app.ConfirmLinkImport(testIngestSessionID, testLinkURL, approved)

	// Then it emits the same ingest:progress/ingest:done events a file import does
	require.NoError(t, err)
	require.Len(t, captured.progress, 2)
	assert.Equal(t, testLinkURL, captured.progress[1].CurrentFile)
	require.NotNil(t, captured.done)
	assert.Equal(t, 1, captured.done.FilesIngested)
	assert.Empty(t, captured.errors)
}

func TestApp_ConfirmLinkImport_emitsError_whenTheURLIsInvalid(t *testing.T) {
	// Given a URL that is not http(s)
	app, captured := newTestIngestApp(t, nil, nil, nil, nil, nil, nil)

	// When confirming it
	err := app.ConfirmLinkImport(testIngestSessionID, "ftp://example.com/a", "content")

	// Then an "ingest:error" event is emitted and the error is returned
	require.ErrorIs(t, err, web.ErrInvalidURL)
	require.Len(t, captured.errors, 1)
	assert.Nil(t, captured.done)
}
