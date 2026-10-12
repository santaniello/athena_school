package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	"github.com/santaniello/athena/internal/domain/web"
	webmocks "github.com/santaniello/athena/internal/domain/web/mocks"
)

const (
	testArticleURL = "https://example.com/article"
	// A passage padded well past the 200-rune chunk floor, so it survives
	// mergeUndersized as its own chunk.
	testArticleFormatted = "## Eigenvectors\n\nAn eigenvector keeps its direction under the transformation, scaled by its eigenvalue — a passage padded well past the 200-rune chunk floor so it survives mergeUndersized as its own chunk."
)

func TestPreviewURL_blankSessionID_isRejectedBeforeAnythingElse(t *testing.T) {
	// Given a blank sessionID and collaborators that would fail the test if touched
	svc := &Service{index: ingestmocks.NewMockIndexGuard(t), fetcher: webmocks.NewMockFetcher(t)}

	// When previewing a link
	preview, err := svc.PreviewURL(context.Background(), "  ", testArticleURL, nil)

	// Then it is rejected
	assert.ErrorIs(t, err, domainknowledge.ErrSessionRequired)
	assert.Equal(t, ImportPreview{}, preview)
}

func TestPreviewURL_invalidURL_isRejectedBeforeFetching(t *testing.T) {
	// Given a fetcher that would fail the test if called
	svc := &Service{index: ingestmocks.NewMockIndexGuard(t), fetcher: webmocks.NewMockFetcher(t)}

	// When previewing something that is not an http(s) URL
	preview, err := svc.PreviewURL(context.Background(), testSessionID, "ftp://example.com/a", nil)

	// Then it is rejected as an invalid URL
	assert.ErrorIs(t, err, web.ErrInvalidURL)
	assert.Equal(t, ImportPreview{}, preview)
}

func TestPreviewURL_returnsTheIndexError_whenTheIndexIsBusy(t *testing.T) {
	// Given an index that refuses mutations
	busy := errors.New("index is retrying")
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(busy)
	svc := &Service{index: guard, fetcher: webmocks.NewMockFetcher(t)}

	// When previewing a link
	_, err := svc.PreviewURL(context.Background(), testSessionID, testArticleURL, nil)

	// Then the index error is returned and nothing is fetched
	assert.ErrorIs(t, err, busy)
}

func TestPreviewURL_propagatesFetchErrors(t *testing.T) {
	// Given a fetcher that reports a blocked host
	fetcher := webmocks.NewMockFetcher(t)
	fetcher.EXPECT().Fetch(mock.Anything, testArticleURL).Return(web.Page{}, web.ErrBlockedHost).Once()
	svc := &Service{index: passingPreviewIndexGuard(t), fetcher: fetcher}

	// When previewing the link
	preview, err := svc.PreviewURL(context.Background(), testSessionID, testArticleURL, nil)

	// Then the typed error reaches the caller and no LLM call is made
	assert.ErrorIs(t, err, web.ErrBlockedHost)
	assert.Equal(t, ImportPreview{}, preview)
}

func TestPreviewURL_returnsFormattedContentAndMatchingChunks(t *testing.T) {
	// Given a fetched article and a provider that cleans it up
	const original = "eigenvectors dont change direction. the eigenvalue is the scale."
	fetcher := webmocks.NewMockFetcher(t)
	fetcher.EXPECT().Fetch(mock.Anything, testArticleURL).
		Return(web.Page{URL: testArticleURL, Title: "Linear Algebra", Markdown: original}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Task == domainllm.TaskNoteFormatting &&
				req.Messages[0].Content == noteFormattingMarkdownPrompt &&
				req.Messages[1].Content == original
		})).
		Return(domainllm.ChatResponse{Content: testArticleFormatted}, nil).Once()
	// No Embeddings expectation: previewing must never embed or persist.
	svc := &Service{index: passingPreviewIndexGuard(t), fetcher: fetcher, llm: llm}

	// When previewing the link
	preview, err := svc.PreviewURL(context.Background(), testSessionID, "https://Example.com/article#top", nil)

	// Then the fetcher is asked for the canonical URL, and the preview carries
	// it, the page title and the exact chunks ChunkMarkdown produces from
	// the formatted text
	require.NoError(t, err)
	assert.Equal(t, testArticleURL, preview.SourceURL)
	assert.Equal(t, "Linear Algebra", preview.DisplayPath)
	assert.Equal(t, original, preview.OriginalContent)
	assert.Equal(t, testArticleFormatted, preview.FormattedContent)
	assert.False(t, preview.Skipped)
	assert.False(t, preview.FormattingFailed)
	require.Len(t, preview.Chunks, 1)
	assert.Equal(t, ChunkMarkdown(testArticleFormatted)[0].Content, preview.Chunks[0].Content)
}

func TestPreviewURL_usesTheURLAsDisplayPath_whenThePageHasNoTitle(t *testing.T) {
	// Given a fetched page without a title
	fetcher := webmocks.NewMockFetcher(t)
	fetcher.EXPECT().Fetch(mock.Anything, testArticleURL).
		Return(web.Page{URL: testArticleURL, Markdown: "text"}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(mock.Anything, mock.Anything).
		Return(domainllm.ChatResponse{Content: "text"}, nil).Once()
	svc := &Service{index: passingPreviewIndexGuard(t), fetcher: fetcher, llm: llm}

	// When previewing the link
	preview, err := svc.PreviewURL(context.Background(), testSessionID, testArticleURL, nil)

	// Then the canonical URL stands in for the title
	require.NoError(t, err)
	assert.Equal(t, testArticleURL, preview.DisplayPath)
}

func TestPreviewURL_reportsFormattingProgress(t *testing.T) {
	// Given a fetched article that formats in one batch
	fetcher := webmocks.NewMockFetcher(t)
	fetcher.EXPECT().Fetch(mock.Anything, testArticleURL).
		Return(web.Page{URL: testArticleURL, Markdown: "text"}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(mock.Anything, mock.Anything).
		Return(domainllm.ChatResponse{Content: "text"}, nil).Once()
	svc := &Service{index: passingPreviewIndexGuard(t), fetcher: fetcher, llm: llm}

	var seen []Progress
	// When previewing the link with a progress callback
	_, err := svc.PreviewURL(context.Background(), testSessionID, testArticleURL, func(p Progress) error {
		seen = append(seen, p)
		return nil
	})

	// Then the section progress is reported like a file preview
	require.NoError(t, err)
	require.Len(t, seen, 1)
	assert.Equal(t, 1, seen[0].SectionsProcessed)
	assert.Equal(t, 1, seen[0].SectionsTotal)
}

func TestConfirmURLImport_blankSessionID_isRejected(t *testing.T) {
	// Given a blank sessionID and a guard that would fail the test if touched
	svc := &Service{index: ingestmocks.NewMockIndexGuard(t)}

	// When confirming a link import
	_, err := svc.ConfirmURLImport(context.Background(), "", testArticleURL, "Title", "approved", nil)

	// Then it is rejected
	assert.ErrorIs(t, err, domainknowledge.ErrSessionRequired)
}

func TestConfirmURLImport_invalidURL_isRejectedBeforeReservingTheIndex(t *testing.T) {
	// Given a guard that would fail the test if asked to reserve the index
	svc := &Service{index: ingestmocks.NewMockIndexGuard(t)}

	// When confirming an import for a URL that is not http(s)
	_, err := svc.ConfirmURLImport(context.Background(), testSessionID, "file:///etc/passwd", "Title", "approved", nil)

	// Then it is rejected as an invalid URL
	assert.ErrorIs(t, err, web.ErrInvalidURL)
}

func TestConfirmURLImport_returnsTheIndexError_whenTheIndexIsBusy(t *testing.T) {
	// Given an index that cannot be reserved
	busy := errors.New("index is retrying")
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(busy)
	svc := &Service{index: guard}

	// When confirming a link import
	_, err := svc.ConfirmURLImport(context.Background(), testSessionID, testArticleURL, "Title", "approved", nil)

	// Then the index error is returned
	assert.ErrorIs(t, err, busy)
}

func TestConfirmURLImport_persistsExactlyTheApprovedContent_underTheCanonicalURL(t *testing.T) {
	// Given approved (formatted) content for a link whose spelling is not canonical
	const approved = "# Linear Algebra\n\nEigenvectors, formatted and approved during review."
	ctx := context.Background()

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, testArticleURL).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 &&
			cs[0].Content == approved &&
			cs[0].SourcePath == testArticleURL &&
			cs[0].FilePath == "Linear Algebra"
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.SourcePath == testArticleURL && f.Path == "Linear Algebra" && f.ChunkCount == 1
	})).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	var seen []Progress
	// When confirming the import
	summary, err := service.ConfirmURLImport(ctx, testSessionID, "HTTPS://Example.com/article#top", "Linear Algebra", approved,
		func(p Progress) error {
			seen = append(seen, p)
			return nil
		})

	// Then exactly the approved text is stored under the canonical URL, and
	// progress ends with the finished source
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	require.NotEmpty(t, seen)
	assert.Equal(t, 1, seen[len(seen)-1].FilesProcessed)
	assert.Equal(t, "Linear Algebra", seen[len(seen)-1].CurrentFile)
}

func TestConfirmURLImport_reAddingTheSameURL_replacesTheExistingSource(t *testing.T) {
	// Given a URL that was already imported as item-1
	const approved = "# Linear Algebra\n\nUpdated content approved during review."
	ctx := context.Background()

	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)

	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{
		testArticleURL: {SessionID: testSessionID, SourcePath: testArticleURL, Path: "Old title", ItemID: "item-1"},
	}, nil).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, testArticleURL).Return([]string{"old-chunk"}, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].ItemID == "item-1"
	})).Return(nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	items.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.ID == "item-1" })).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.ItemID == "item-1" && f.Path == "New title"
	})).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"old-chunk"}).Return(nil).Once()
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, store, passingIndexGuard(t))

	// When confirming the same URL again with a new title
	summary, err := service.ConfirmURLImport(ctx, testSessionID, testArticleURL, "New title", approved, nil)

	// Then the existing item is reused (nothing is duplicated) and the title updated
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
}

func TestConfirmURLImport_usesTheURLAsDisplayPath_whenTheTitleIsBlank(t *testing.T) {
	// Given approved content and a blank title
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, testArticleURL).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.Path == testArticleURL
	})).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))

	// When confirming the import
	_, err := service.ConfirmURLImport(ctx, testSessionID, testArticleURL, "  ", "Some approved text.", nil)

	// Then the URL is the stored display path
	require.NoError(t, err)
}

func TestConfirmURLImport_stopsWhenTheProgressCallbackFails(t *testing.T) {
	// Given a progress callback that fails (the UI is gone)
	gone := errors.New("ui gone")
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	llm := llmmocks.NewMockProvider(t)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, mock.Anything).Return(embeddingResponse(), nil).Once()
	service := newTestService(nil, ingestedFiles, nil, nil, llm, nil, nil, passingIndexGuard(t))

	// When confirming the import
	_, err := service.ConfirmURLImport(ctx, testSessionID, testArticleURL, "Title", "Some approved text.",
		func(Progress) error { return gone })

	// Then the callback error propagates and nothing is persisted
	assert.ErrorIs(t, err, gone)
}
