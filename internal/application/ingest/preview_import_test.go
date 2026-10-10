package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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
