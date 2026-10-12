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

func TestFormatBatch_returnsTheFormattedContent_onSuccess(t *testing.T) {
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

	// When formatting the batch
	formatted, failed := svc.formatBatch(context.Background(), testSessionID, "", "raw messy note", true)

	// Then it returns the rewritten text, not degraded
	assert.Equal(t, "# Clean note\n\nTidy.", formatted)
	assert.False(t, failed)
}

func TestFormatBatch_usesAMarkdownPrompt_whenTheSourceIsMarkdown(t *testing.T) {
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

	// When formatting a .md batch with no section breadcrumb
	_, failed := svc.formatBatch(context.Background(), testSessionID, "", "content", true)

	// Then it used the Markdown-specific prompt unchanged and did not fall back
	assert.False(t, failed)
}

func TestFormatBatch_usesAPlainTextPrompt_whenTheSourceIsNotMarkdown(t *testing.T) {
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

	// When formatting a .txt batch with no section breadcrumb
	_, failed := svc.formatBatch(context.Background(), testSessionID, "", "content", false)

	// Then it used the plain-text prompt (never asking for Markdown syntax
	// ChunkText would not interpret anyway) and did not fall back
	assert.False(t, failed)
}

func TestFormatBatch_foldsTheBreadcrumbIntoTheSystemPrompt_neverTheUserMessage(t *testing.T) {
	// Given a provider that captures both messages it received
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Messages[0].Role == "system" &&
				strings.Contains(req.Messages[0].Content, "Arquitetura > Monolito") &&
				req.Messages[1].Content == "content"
		})).
		Return(domainllm.ChatResponse{Content: "ok"}, nil).
		Once()
	svc := &Service{llm: llm}

	// When formatting a batch under a non-empty breadcrumb
	_, failed := svc.formatBatch(context.Background(), testSessionID, "Arquitetura > Monolito", "content", true)

	// Then the breadcrumb was given as context in the system prompt, and
	// the user message stayed exactly the batch's own content
	assert.False(t, failed)
}

func TestFormatBatch_fallsBackToTheOriginalContent_onProviderError(t *testing.T) {
	// Given a provider whose Chat call fails
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == testSessionID && req.Task == domainllm.TaskNoteFormatting
	})).
		Return(domainllm.ChatResponse{}, errors.New("rate limited")).Once()
	svc := &Service{llm: llm}

	// When formatting the batch
	formatted, failed := svc.formatBatch(context.Background(), testSessionID, "", "original content", true)

	// Then it degrades to the original content instead of blocking the
	// import over a transient formatting failure
	assert.Equal(t, "original content", formatted)
	assert.True(t, failed)
}

func TestFormatBatch_fallsBackToTheOriginalContent_onBlankResponse(t *testing.T) {
	// Given a provider that returns a whitespace-only response
	for _, blank := range []string{"", "   ", "\n\t "} {
		llm := llmmocks.NewMockProvider(t)
		llm.EXPECT().Chat(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.SessionID == testSessionID && req.Task == domainllm.TaskNoteFormatting
		})).
			Return(domainllm.ChatResponse{Content: blank}, nil).Once()
		svc := &Service{llm: llm}

		// When formatting the batch
		formatted, failed := svc.formatBatch(context.Background(), testSessionID, "", "original content", true)

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
	preview, err := svc.PreviewImport(context.Background(), "   ", fstest.MapFS{}, testSourceRoot, "note.md", nil)

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
	_, err := svc.PreviewImport(context.Background(), testSessionID, fstest.MapFS{}, testSourceRoot, "note.pdf", nil)

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
	preview, err := svc.PreviewImport(context.Background(), testSessionID, fstest.MapFS{}, testSourceRoot, "note.md", nil)

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
	preview, err := svc.PreviewImport(context.Background(), testSessionID, realRoot, testSourceRoot, "go.md", nil)

	// Then it reports the file as skipped, with no chunks/formatted text
	require.NoError(t, err)
	assert.True(t, preview.Skipped)
	assert.Equal(t, "go.md", preview.DisplayPath)
	assert.Empty(t, preview.Chunks)
}

func TestPreviewImport_unchangedFile_whenItemLookupFailsForAnotherReason_wrapsTheUnderlyingError(t *testing.T) {
	// Given a file that would otherwise be skipped, but whose restoration
	// check hits a genuine repository error (not ErrItemNotFound)
	const content = "# Go\nBasics of Go."

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	prev := domainknowledge.IngestedFile{
		SessionID: testSessionID, SourcePath: srcPath("go.md"), Path: "go.md",
		MTimeUnixNano: fixedModTime.UnixNano(), EmbeddingModel: domainllm.EmbeddingModel, ItemID: "item-1",
	}
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{srcPath("go.md"): prev}, nil).Once()

	boom := errors.New("database unavailable")
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(mock.Anything, "item-1").Return(domainknowledge.Item{}, boom).Once()

	svc := &Service{
		index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, items: items,
	}
	realRoot := fstest.MapFS{"go.md": {Data: []byte(content), ModTime: fixedModTime}}

	// When previewing an import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, realRoot, testSourceRoot, "go.md", nil)

	// Then the underlying repository error is still reachable via
	// errors.Is, not just flattened into a string
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, ImportPreview{}, preview)
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
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

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
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.txt", nil)

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
	llm.EXPECT().Chat(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == testSessionID && req.Task == domainllm.TaskNoteFormatting
	})).
		Return(domainllm.ChatResponse{}, errors.New("rate limited")).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

	// Then it never blocks the import: it reports the failure and chunks
	// the original, unformatted text instead
	require.NoError(t, err)
	assert.True(t, preview.FormattingFailed)
	assert.Equal(t, original, preview.FormattedContent)
	require.Len(t, preview.Chunks, 1)
	assert.Equal(t, original, preview.Chunks[0].Content)
}

// Diagnosed from a real study session (Day 05 - Estratégias de Cache.md,
// see the "Cache" session's message_sources): LRU/LFU/FIFO/RR were all
// listed as a single tight bulleted list with no blank line between
// items, so chunking-by-paragraph (2.3.1) kept all four in one chunk —
// "LRU" and "LFU" scored high enough to clear Sufficiency against that
// diluted chunk, but "FIFO" and "Random Replacement" did not, producing
// "Nenhum conhecimento local foi encontrado" for two policies covered by
// the very same imported note. Both prompts must explicitly ask the
// formatter to split this pattern — "turn ad-hoc lists into proper
// Markdown bullets" alone does not, since the note's bullets were already
// proper Markdown syntax; the problem was never syntax, it was four
// distinct definitions sharing one paragraph.
func TestPreviewImport_multiSectionDocument_formatsEachSectionInItsOwnCall(t *testing.T) {
	// Given a document with two heading sections, each big enough on its
	// own to force a separate formatting batch
	bodyOne := exactlyNChars(maxFormattingChars)
	bodyTwo := exactlyNChars(maxFormattingChars)
	original := "## One\n" + bodyOne + "\n\n## Two\n" + bodyTwo
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return strings.Contains(req.Messages[1].Content, "One") && !strings.Contains(req.Messages[1].Content, "Two")
		})).
		Return(domainllm.ChatResponse{Content: "## One (rewritten)\n\n" + bodyOne}, nil).Once()
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return strings.Contains(req.Messages[1].Content, "Two") && !strings.Contains(req.Messages[1].Content, "One")
		})).
		Return(domainllm.ChatResponse{Content: "## Two (rewritten)\n\n" + bodyTwo}, nil).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

	// Then each section was sent to the LLM as its own call (asserted via
	// the two distinct EXPECT()s above, each matched to exactly one
	// section's own content), and both rewrites made it into the
	// reassembled formatted text, in original document order
	require.NoError(t, err)
	assert.False(t, preview.FormattingFailed)
	assert.Contains(t, preview.FormattedContent, "## One (rewritten)")
	assert.Contains(t, preview.FormattedContent, "## Two (rewritten)")
	assert.Less(t,
		strings.Index(preview.FormattedContent, "One (rewritten)"),
		strings.Index(preview.FormattedContent, "Two (rewritten)"),
	)
}

func TestPreviewImport_oneSectionFailsToFormat_fallsBackOnlyThatSection_othersStillRewritten(t *testing.T) {
	// Given a document with two heading sections, each big enough on its
	// own to force a separate formatting batch, where only the second
	// section's Chat call fails
	bodyOne := exactlyNChars(maxFormattingChars)
	bodyTwo := exactlyNChars(maxFormattingChars)
	original := "## One\n" + bodyOne + "\n\n## Two\n" + bodyTwo
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return strings.Contains(req.Messages[1].Content, "One") && !strings.Contains(req.Messages[1].Content, "Two")
		})).
		Return(domainllm.ChatResponse{Content: "## One (rewritten)\n\n" + bodyOne}, nil).Once()
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return strings.Contains(req.Messages[1].Content, "Two") && !strings.Contains(req.Messages[1].Content, "One")
		})).
		Return(domainllm.ChatResponse{}, errors.New("timeout")).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

	// Then the failure is reported, but only the second section fell back
	// to its own original content — the first section's rewrite survives,
	// unlike the old whole-document formatNote, where one failing call
	// discarded every section's formatting
	require.NoError(t, err)
	assert.True(t, preview.FormattingFailed)
	assert.Contains(t, preview.FormattedContent, "## One (rewritten)")
	assert.Contains(t, preview.FormattedContent, "## Two\n"+bodyTwo)
	assert.NotContains(t, preview.FormattedContent, "## Two (rewritten)")
	// And FailedSections names exactly the failing section, not "One"
	require.Len(t, preview.FailedSections, 1)
	assert.Equal(t, "Two", preview.FailedSections[0].Heading)
	assert.NotEmpty(t, preview.FailedSections[0].Reason)
}

func TestPreviewImport_formatterTranslatesTheNote_fidelityValidationFallsBackToTheOriginal(t *testing.T) {
	// Given the real failure mode diagnosed against a BFF-style note: a
	// fluent, non-empty Chat response that silently translated a
	// Portuguese note into English, long enough to carry a real language
	// signal and padded well past minChunkChars
	const original = "Este padrão reduz o blast radius de cada cliente ao isolar falhas no backend " +
		"compartilhado, evitando que uma falha em um serviço afete todos os consumidores da API " +
		"de forma inesperada."
	const translated = "This pattern reduces the blast radius of each client by isolating failures in " +
		"the shared backend, preventing a failure in one service from unexpectedly affecting every " +
		"consumer of the API."
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == testSessionID && req.Task == domainllm.TaskNoteFormatting
	})).
		Return(domainllm.ChatResponse{Content: translated}, nil).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

	// Then fidelity validation rejects the translated rewrite — the
	// non-empty response alone is not enough to accept it — and the
	// preview falls back to the original Portuguese text, flagged as a
	// formatting failure instead of silently persisting a translation
	require.NoError(t, err)
	assert.True(t, preview.FormattingFailed)
	assert.Equal(t, original, preview.FormattedContent)
	assert.NotContains(t, preview.FormattedContent, "blast radius of each client")
}

func TestPreviewImport_namedListStillCollapsed_getsOneCorrectiveRetry_thatSucceeds(t *testing.T) {
	// Given the real LRU/LFU/FIFO/RR shape: a tight bulleted list with no
	// blank line between items — the document's only section, so it is
	// also its own batch regardless of size
	original := "### Políticas de Evicção\n" +
		"- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n- RR: remove aleatoriamente.\n"
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	stillCollapsed := "### Políticas de Evicção\n\n" +
		"- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n- RR: remove aleatoriamente.\n"
	splitApart := "### Políticas de Evicção\n\n" +
		"LRU (Least Recently Used) remove o item " + exactlyNChars(180) + ".\n\n" +
		"LFU (Least Frequently Used) remove o item " + exactlyNChars(180) + ".\n\n" +
		"FIFO (First In First Out) remove o item " + exactlyNChars(180) + ".\n\n" +
		"RR (Random Replacement) remove o item " + exactlyNChars(180) + "."

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	// First attempt: no corrective instruction yet — returns a rewrite
	// that still reads as one collapsed block.
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return !strings.Contains(req.Messages[0].Content, "still reads as one block")
		})).
		Return(domainllm.ChatResponse{Content: stillCollapsed}, nil).Once()
	// Corrective retry: the one bounded follow-up — returns a rewrite that
	// actually splits the list apart.
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return strings.Contains(req.Messages[0].Content, "still reads as one block")
		})).
		Return(domainllm.ChatResponse{Content: splitApart}, nil).Once()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

	// Then the corrective retry's split-apart rewrite is what gets used —
	// not the first attempt's still-collapsed one — and no formatting
	// failure is reported, since the retry succeeded
	require.NoError(t, err)
	assert.False(t, preview.FormattingFailed)
	assert.Contains(t, preview.FormattedContent, "Least Recently Used")
	assert.True(t, len(preview.Chunks) >= 4, "expected at least 4 chunks, got %d", len(preview.Chunks))
}

func TestPreviewImport_namedListStillCollapsed_correctiveRetryAlsoFails_keepsFirstRewriteFlaggedNotFallback(t *testing.T) {
	// Given the same collapsed-list shape, where the corrective retry
	// comes back just as collapsed as the first attempt
	original := "### Políticas de Evicção\n" +
		"- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n- RR: remove aleatoriamente.\n"
	root := fstest.MapFS{"note.md": {Data: []byte(original)}}

	stillCollapsed := "### Políticas de Evicção (revisado)\n\n" +
		"- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n- RR: remove aleatoriamente.\n"

	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()

	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
		return req.SessionID == testSessionID && req.Task == domainllm.TaskNoteFormatting
	})).
		Return(domainllm.ChatResponse{Content: stillCollapsed}, nil).Twice()

	svc := &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm}

	// When previewing the import
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "note.md", nil)

	// Then the first rewrite is still used — it passed fidelity validation
	// and is a legitimate rewrite, just not a structural win for this list
	// — and this is NOT reported as a formatting failure: the structural
	// signal is informational (surfaced in a later UI pass), never a
	// fallback trigger on its own
	require.NoError(t, err)
	assert.False(t, preview.FormattingFailed)
	assert.Empty(t, preview.FailedSections)
	assert.Contains(t, preview.FormattedContent, "Políticas de Evicção (revisado)")
	// And the still-collapsed list is surfaced as a structure warning,
	// not silent success
	require.Len(t, preview.StructureWarnings, 1)
	assert.Equal(t, "Políticas de Evicção", preview.StructureWarnings[0].Heading)
	assert.NotEmpty(t, preview.StructureWarnings[0].Message)
}

func TestNoteFormattingPrompts_coverBundledListDefinitions(t *testing.T) {
	for _, prompt := range []string{noteFormattingMarkdownPrompt, noteFormattingPlainTextPrompt} {
		assert.Contains(t, prompt, "blank line")
		// A blank line alone isn't enough: a one-line bullet
		// ("- FIFO: removes the oldest item") is well under minChunkChars
		// (200 runes), so mergeUndersized cascades it straight back into
		// its neighbours regardless of the blank line between them —
		// verified against the real OpenRouter API, which DID add blank
		// lines on the prompt's first version but still produced 1 chunk,
		// not 4, for exactly this reason. The prompt must also ask for
		// each item to be expanded into its own explanation, long enough
		// to clear that floor on its own.
		assert.Contains(t, prompt, "sentence")
	}
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
