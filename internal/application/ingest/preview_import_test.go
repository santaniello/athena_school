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

func TestFormatNote_returnsTheFormattedContent_onSuccess(t *testing.T) {
	// Given a provider that rewrites the note into clean Markdown
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.SessionID == testSessionID &&
				req.Task == domainllm.TaskNoteFormatting &&
				len(req.Messages) == 2 &&
				req.Messages[1].Content == "raw messy note"
		})).
		Return(domainllm.ChatResponse{Content: "# Clean note\n\nTidy."}, nil).
		Once()
	svc := &Service{llm: llm}

	// When formatting the note
	formatted, failed := svc.formatNote(context.Background(), testSessionID, "raw messy note", true)

	// Then it returns the rewritten text, not degraded
	assert.Equal(t, "# Clean note\n\nTidy.", formatted)
	assert.False(t, failed)
}

func TestFormatNote_usesAMarkdownPrompt_whenTheSourceIsMarkdown(t *testing.T) {
	// Given a provider that captures the system prompt it received
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Messages[0].Role == "system" &&
				req.Messages[0].Content == noteFormattingMarkdownPrompt
		})).
		Return(domainllm.ChatResponse{Content: "ok"}, nil).
		Once()
	svc := &Service{llm: llm}

	// When formatting a .md source
	_, failed := svc.formatNote(context.Background(), testSessionID, "content", true)

	// Then it used the Markdown-specific prompt and did not fall back
	assert.False(t, failed)
}

func TestFormatNote_usesAPlainTextPrompt_whenTheSourceIsNotMarkdown(t *testing.T) {
	// Given a provider that captures the system prompt it received
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Messages[0].Role == "system" &&
				req.Messages[0].Content == noteFormattingPlainTextPrompt
		})).
		Return(domainllm.ChatResponse{Content: "ok"}, nil).
		Once()
	svc := &Service{llm: llm}

	// When formatting a .txt source
	_, failed := svc.formatNote(context.Background(), testSessionID, "content", false)

	// Then it used the plain-text prompt (never asking for Markdown syntax
	// ChunkText would not interpret anyway) and did not fall back
	assert.False(t, failed)
}

func TestFormatNote_fallsBackToTheOriginalContent_onProviderError(t *testing.T) {
	// Given a provider whose Chat call fails
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(mock.Anything, mock.Anything).
		Return(domainllm.ChatResponse{}, errors.New("rate limited")).Once()
	svc := &Service{llm: llm}

	// When formatting the note
	formatted, failed := svc.formatNote(context.Background(), testSessionID, "original content", true)

	// Then it degrades to the original content instead of blocking the
	// import over a transient formatting failure
	assert.Equal(t, "original content", formatted)
	assert.True(t, failed)
}

func TestFormatNote_fallsBackToTheOriginalContent_onBlankResponse(t *testing.T) {
	// Given a provider that returns a whitespace-only response
	for _, blank := range []string{"", "   ", "\n\t "} {
		llm := llmmocks.NewMockProvider(t)
		llm.EXPECT().Chat(mock.Anything, mock.Anything).
			Return(domainllm.ChatResponse{Content: blank}, nil).Once()
		svc := &Service{llm: llm}

		// When formatting the note
		formatted, failed := svc.formatNote(context.Background(), testSessionID, "original content", true)

		// Then it degrades to the original content rather than persisting
		// a blank/degenerate rewrite
		require.Equal(t, "original content", formatted, "content = %q", blank)
		assert.True(t, failed, "content = %q", blank)
	}
}

func passingPreviewIndexGuard(t *testing.T) *ingestmocks.MockIndexGuard {
	guard := ingestmocks.NewMockIndexGuard(t)
	guard.EXPECT().CheckMutationAllowed().Return(nil)
	return guard
}

func TestPreviewImport_blankSessionID_isRejectedBeforeCheckingTheIndex(t *testing.T) {
	// Given a blank sessionID and a guard that would fail the test if asked
	svc := &Service{index: ingestmocks.NewMockIndexGuard(t)}

	// When previewing an import
	preview, err := svc.PreviewImport(context.Background(), "   ", fstest.MapFS{}, testSourceRoot, "note.md")

	// Then it is rejected the same way ImportFile rejects it, before ever
	// touching the index guard
	require.Error(t, err)
	assert.ErrorIs(t, err, domainknowledge.ErrSessionRequired)
	assert.Equal(t, ImportPreview{}, preview)
}

func TestPreviewImport_rejectsUnsupportedExtension_beforeCheckingTheIndex(t *testing.T) {
	// Given an unsupported file extension
	svc := &Service{index: ingestmocks.NewMockIndexGuard(t)}

	// When previewing an import
	_, err := svc.PreviewImport(context.Background(), testSessionID, fstest.MapFS{}, testSourceRoot, "note.pdf")

	// Then it is rejected before ever touching the index guard
	require.Error(t, err)
}

func TestPreviewImport_returnsTheIndexError_whenTheIndexIsBusy(t *testing.T) {
	// Given an index guard that reports it is busy
	busyErr := errors.New("index is loading")
	index := ingestmocks.NewMockIndexGuard(t)
	index.EXPECT().CheckMutationAllowed().Return(busyErr)
	svc := &Service{index: index}

	// When previewing an import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, fstest.MapFS{}, testSourceRoot, "note.md")

	// Then the busy error is propagated, and nothing else is touched —
	// note there is no EXPECT() for BeginMutation/EndMutation: PreviewImport
	// never reserves the index (spec 2.20 decision 9), only checks it
	require.Error(t, err)
	assert.ErrorIs(t, err, busyErr)
	assert.Equal(t, ImportPreview{}, preview)
}

func TestPreviewImport_unchangedFile_isSkipped_withoutCallingTheFormatter(t *testing.T) {
	// Given a file whose mtime and embedding model already match its last
	// import, and whose Item/document both still exist
	const content = "# Go\nBasics of Go."

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	prev := domainknowledge.IngestedFile{
		SessionID: testSessionID, SourcePath: srcPath("go.md"), Path: "go.md",
		MTimeUnixNano: fixedModTime.UnixNano(), EmbeddingModel: domainllm.EmbeddingModel, ItemID: "item-1",
	}
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{srcPath("go.md"): prev}, nil).Once()

	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(mock.Anything, testSessionID, "item-1").Return("stored text", nil).Once()

	// llm has no EXPECT() set up at all: if PreviewImport called Chat
	// anyway, the mock would fail the test — proving the formatter is
	// never invoked for an unchanged file.
	llm := llmmocks.NewMockProvider(t)

	svc := &Service{
		index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, items: items, documents: documents, llm: llm,
	}
	realRoot := fstest.MapFS{"go.md": {Data: []byte(content), ModTime: fixedModTime}}

	// When previewing an import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, realRoot, testSourceRoot, "go.md")

	// Then it reports the file as skipped, with no chunks/formatted text
	require.NoError(t, err)
	assert.True(t, preview.Skipped)
	assert.Equal(t, "go.md", preview.DisplayPath)
	assert.Empty(t, preview.Chunks)
}

func TestPreviewImport_newMarkdownFile_returnsFormattedContentAndMatchingChunks(t *testing.T) {
	// Given a never-before-seen markdown file and a provider that cleans it up
	const original = "eigenvectors dont change direction. the eigenvalue is the scale."
	const formattedText = "## Eigenvectors\n\nAn eigenvector keeps its direction under the transformation, scaled by its eigenvalue — a passage padded well past the 200-rune chunk floor so it survives mergeUndersized as its own chunk, exactly as this test expects."
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Task == domainllm.TaskNoteFormatting && req.Messages[1].Content == original
		})).
		Return(domainllm.ChatResponse{Content: formattedText}, nil).Once()
	// No EXPECT() for Embeddings: PreviewImport must never embed (spec 2.20
	// decision 5) — a call to it would fail this test.

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md")

	// Then it returns the formatted text and the exact chunks ChunkMarkdown
	// would produce from it — what ConfirmImport will later persist
	require.NoError(t, err)
	assert.False(t, preview.Skipped)
	assert.False(t, preview.FormattingFailed)
	assert.Equal(t, original, preview.OriginalContent)
	assert.Equal(t, formattedText, preview.FormattedContent)
	require.Len(t, preview.Chunks, 1)
	assert.Equal(t, ChunkMarkdown(formattedText)[0].Content, preview.Chunks[0].Content)
	assert.Equal(t, "Eigenvectors", preview.Chunks[0].Heading)
	assert.Positive(t, preview.Chunks[0].CharCount)
	assert.Equal(t, preview.Chunks[0].CharCount/4, preview.Chunks[0].EstimatedTokens)
}

func TestPreviewImport_txtFile_usesChunkTextAndThePlainTextPrompt(t *testing.T) {
	// Given a never-before-seen .txt file
	const original = "a long plain-text note with no headings at all, well past the chunk floor."
	root := fstest.MapFS{"note.txt": {Data: []byte(original)}}

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Messages[0].Content == noteFormattingPlainTextPrompt
		})).
		Return(domainllm.ChatResponse{Content: original}, nil).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.txt")

	// Then it chunked with ChunkText (no heading), matching a .txt import
	require.NoError(t, err)
	require.Len(t, preview.Chunks, 1)
	assert.Empty(t, preview.Chunks[0].Heading)
}

func TestPreviewImport_formattingFails_fallsBackToChunkingTheOriginalContent(t *testing.T) {
	// Given a provider whose Chat call fails
	const original = "a note the formatter could not reach, long enough to survive as its own chunk on its own."
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(mock.Anything, mock.Anything).
		Return(domainllm.ChatResponse{}, errors.New("rate limited")).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md")

	// Then it never blocks the import: it reports the failure and chunks
	// the original, unformatted text instead
	require.NoError(t, err)
	assert.True(t, preview.FormattingFailed)
	assert.Equal(t, original, preview.FormattedContent)
	require.Len(t, preview.Chunks, 1)
	assert.Equal(t, original, preview.Chunks[0].Content)
}

func TestComputeFormattingStats_countsHeadingsAndParagraphsBeforeAndAfter(t *testing.T) {
	// Given a messy, heading-less original and a formatted version split
	// into one heading and two paragraphs
	original := "eigenvectors are vectors that dont change direction. the eigenvalue is the scale factor."
	formatted := "## Eigenvectors\n\nAn eigenvector keeps its direction under the transformation.\n\nIts eigenvalue is the scale factor applied to it."

	// When computing the before/after structural stats
	stats := computeFormattingStats(original, formatted)

	// Then it reports real counts, not a narrated claim
	assert.Equal(t, FormattingStats{
		HeadingsBefore:   0,
		HeadingsAfter:    1,
		ParagraphsBefore: 1,
		ParagraphsAfter:  2,
	}, stats)
}
