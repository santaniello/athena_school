package ingest

import (
	"context"
	"strings"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// noteFormattingMarkdownPrompt asks the model to rewrite a .md source into
// clean Markdown that chunks well: normalized heading hierarchy, one
// paragraph per sub-topic, proper bullet syntax, export artifacts stripped.
// It must preserve meaning, facts, numbers and language exactly — this is a
// rewrite for structure, never a summary. See
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
const noteFormattingMarkdownPrompt = "Rewrite the note below as clean Markdown to improve retrieval chunking: normalize heading levels (H1-H3), split run-on prose into one paragraph per sub-topic, turn ad-hoc \"-\"/\"*\" lists into proper Markdown bullets, and strip page-break or export artifacts. Preserve the original meaning, facts, numbers and language exactly — never summarize, add, or remove information. Output only the rewritten Markdown, nothing else."

// noteFormattingPlainTextPrompt is noteFormattingMarkdownPrompt's
// counterpart for a .txt source: ChunkText never interprets Markdown
// syntax, so asking for headings/bullets here would just leave literal
// "##"/"-" characters sitting in the stored text. It only asks for
// well-structured plain-text paragraphs.
const noteFormattingPlainTextPrompt = "Rewrite the note below as well-structured plain text to improve retrieval chunking: split run-on prose into one paragraph per sub-topic, and strip page-break or export artifacts. Do not use Markdown syntax (no \"#\" headings, no \"-\"/\"*\" bullets) — plain paragraphs only. Preserve the original meaning, facts, numbers and language exactly — never summarize, add, or remove information. Output only the rewritten text, nothing else."

// FormattingStats are deterministic, verifiable structure counts comparing
// a note's original and formatted text — never a model's own narrated
// claim about what it changed, which the pipeline has no way to verify.
type FormattingStats struct {
	HeadingsBefore   int
	HeadingsAfter    int
	ParagraphsBefore int
	ParagraphsAfter  int
}

// formatNote rewrites content into clean Markdown (isMarkdown) or
// well-structured plain text (!isMarkdown) via one non-streamed Chat call.
// A Provider error or a blank response degrades to the original content
// (failed = true) instead of blocking the import over a transient
// formatting failure — mirroring HydeEnricher.Enrich's blank-response
// guard (2.5.1), but falling back instead of erroring.
func (s *Service) formatNote(ctx context.Context, sessionID, content string, isMarkdown bool) (formatted string, failed bool) {
	prompt := noteFormattingPlainTextPrompt
	if isMarkdown {
		prompt = noteFormattingMarkdownPrompt
	}

	resp, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskNoteFormatting,
		Messages: []domainllm.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: content},
		},
	})
	if err != nil {
		return content, true
	}

	cleaned := strings.TrimSpace(resp.Content)
	if cleaned == "" {
		return content, true
	}
	return cleaned, false
}

// computeFormattingStats reports headingsBefore/After and paragraphsBefore/
// After for original vs formatted, reusing the same heading/paragraph
// parsing chunking.go already does — never a separately-maintained count.
func computeFormattingStats(original, formatted string) FormattingStats {
	return FormattingStats{
		HeadingsBefore:   countHeadings(original),
		HeadingsAfter:    countHeadings(formatted),
		ParagraphsBefore: countParagraphs(original),
		ParagraphsAfter:  countParagraphs(formatted),
	}
}

// countHeadings returns the number of H1-H3 headings findHeadingSections
// detects in source — 0 for a document with none (including a .txt
// source, which never has Markdown headings).
func countHeadings(source string) int {
	sections := findHeadingSections(source)
	count := 0
	for _, section := range sections {
		if section.Heading != "" {
			count++
		}
	}
	return count
}

// countParagraphs returns the number of blank-line-separated paragraphs
// splitParagraphs detects across the whole source, excluding a paragraph
// that is nothing but a single Markdown heading line (e.g. "## Eigenvectors")
// — findHeadingSections' Content includes its own heading line, so
// splitParagraphs would otherwise count that line itself as a body
// paragraph, which it never is. A .txt source has no heading lines, so this
// exclusion is a no-op there.
func countParagraphs(source string) int {
	count := 0
	for _, p := range splitParagraphs(source) {
		if isHeadingLineOnly(p.Text) {
			continue
		}
		count++
	}
	return count
}

// isHeadingLineOnly reports whether text is nothing but a single Markdown
// heading line: starts with "#" and contains no other line.
func isHeadingLineOnly(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed, "\n")
}
