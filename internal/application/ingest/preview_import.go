package ingest

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// noteFormattingMarkdownPrompt asks the model to rewrite a .md source into
// clean Markdown that chunks well: normalized heading hierarchy, one
// paragraph per sub-topic, proper bullet syntax, export artifacts stripped
// — and, critically, each item of a list that enumerates several distinct
// terms gets the list's shared context folded into its own sentence, with
// a blank line between items.
//
// Diagnosed from a real session (see TestNoteFormattingPrompts_
// coverBundledListDefinitions, which caught v1 of this prompt failing a
// real API call): a cache note's eviction-policy bullets (LRU/LFU/FIFO/RR)
// had no blank line between them, so the whole list chunked as one block;
// "FIFO"/"Random Replacement" scored below Sufficiency against it even
// though "LRU"/"LFU" happened to clear the bar — generic terms (FIFO, a
// queueing concept far outside caching too) dilute less cleanly than a
// specific CS term already tightly tied to "cache" in embedding space. A
// first fix (blank line only) was verified against the real OpenRouter API
// and still produced 1 chunk, not 4: a one-line bullet like "- FIFO:
// remove o mais antigo" is nowhere near minChunkChars (200 runes), so
// mergeUndersized (chunking.go) cascades every short item straight back
// into its neighbours regardless of the blank line between them. Asking
// the model to pad each item with invented explanation would violate the
// very next sentence below ("never add information") — instead, each item
// pulls in context already stated once in the list's own heading/intro, so
// it is self-contained and long enough to stand alone without introducing
// a single new fact. See
// specs/phases/phase-02-knowledge-engine/20-import-review-and-formatting.md.
const noteFormattingMarkdownPrompt = "Rewrite the note below as clean Markdown to improve retrieval chunking: normalize heading levels (H1-H3), split run-on prose into one paragraph per sub-topic, and turn ad-hoc \"-\"/\"*\" lists into proper Markdown bullets. When a list enumerates several distinct, separately-definable items under a shared heading or intro sentence (e.g. one named policy, term or algorithm per item) — restate that shared context inside each item's own sentence, not just in the surrounding heading, so every item is self-contained and long enough to stand as its own chunk, with a blank line between every item; never leave two or more distinct definitions bundled into one paragraph just because they share a list, and never invent an explanation the note does not already give — only make existing nearby context explicit within each item. Strip page-break or export artifacts. Preserve the original meaning, facts, numbers and language exactly — never summarize, add, or remove information. Output only the rewritten Markdown, nothing else."

// noteFormattingPlainTextPrompt is noteFormattingMarkdownPrompt's
// counterpart for a .txt source: ChunkText never interprets Markdown
// syntax, so asking for headings/bullets here would just leave literal
// "##"/"-" characters sitting in the stored text. It only asks for
// well-structured plain-text paragraphs, with the same contextual-
// restatement rule (see noteFormattingMarkdownPrompt's comment for why).
const noteFormattingPlainTextPrompt = "Rewrite the note below as well-structured plain text to improve retrieval chunking: split run-on prose into one paragraph per sub-topic. When a list enumerates several distinct, separately-definable items under a shared heading or intro sentence (e.g. one named policy, term or algorithm per item) — restate that shared context inside each item's own sentence, not just in the surrounding heading, so every item is self-contained and long enough to stand as its own chunk, with a blank line between every item; never leave two or more distinct definitions bundled into one paragraph just because they share a list, and never invent an explanation the note does not already give — only make existing nearby context explicit within each item. Strip page-break or export artifacts. Do not use Markdown syntax (no \"#\" headings, no \"-\"/\"*\" bullets) — plain paragraphs only. Preserve the original meaning, facts, numbers and language exactly — never summarize, add, or remove information. Output only the rewritten text, nothing else."

// FormattingStats are deterministic, verifiable structure counts comparing
// a note's original and formatted text — never a model's own narrated
// claim about what it changed, which the pipeline has no way to verify.
type FormattingStats struct {
	HeadingsBefore   int
	HeadingsAfter    int
	ParagraphsBefore int
	ParagraphsAfter  int
}

// ChunkPreview is one would-be chunk shown to the user before import is
// approved — the exact text ConfirmImport's chunker will also produce from
// the same approved content, since both run the same ChunkMarkdown/
// ChunkText over it (spec 2.20 decision 4).
type ChunkPreview struct {
	Heading         string
	Content         string
	CharCount       int
	EstimatedTokens int
}

// FailedSection describes one formatting batch that fell back to its own
// original content, so the Review dialog can tell the user which part of
// the note was affected instead of just that FormattingFailed is true.
type FailedSection struct {
	Heading string
	Reason  string
}

// StructureWarning flags one batch whose rewrite was accepted (it passed
// fidelity validation — this is not a FailedSection) but still left a
// detected named list collapsed into one chunk even after one corrective
// retry (see validateFormattingStructure). Never computed as a reason to
// fall back, only as a visible, honest signal in place of silent success.
type StructureWarning struct {
	Heading string
	Message string
}

// ImportPreview is PreviewImport's result. Skipped is true when the file is
// unchanged since its last import (same mtime, same embedding model) — in
// that case FormattedContent/Chunks are empty and the caller should not
// open the review dialog at all, the same silent-skip behavior ImportFile
// already has.
type ImportPreview struct {
	DisplayPath      string
	OriginalContent  string
	FormattedContent string
	// FormattingFailed is true when at least one formatting batch degraded
	// to its own original content instead of a rewrite (see formatBatches,
	// anyBatchFellBack).
	FormattingFailed bool
	// FailedSections lists each batch behind FormattingFailed, by heading
	// and reason. Empty when FormattingFailed is false.
	FailedSections []FailedSection
	// StructureWarnings lists each accepted batch whose chunking outcome
	// still looks unhealthy (see StructureWarning) — informational, not a
	// subset of FailedSections.
	StructureWarnings []StructureWarning
	Stats             FormattingStats
	Chunks            []ChunkPreview
	Skipped           bool
	SkippedReason     string
}

// PreviewImport reformats and chunks filePath without persisting anything —
// no embeddings, no database write, no VectorStore mutation. ConfirmImport
// is PreviewImport's only valid follow-up: it must receive exactly the
// FormattedContent this returns, never re-read or re-format the file
// itself, so what the user reviews here is byte-for-byte what gets
// embedded. Unlike ImportFile, this only checks — it never reserves — the
// index: Preview and the eventual Confirm are two separate round-trips with
// arbitrary user think-time in between, so holding BeginMutation open across
// that gap would block unrelated index operations for as long as the review
// dialog sits open (spec 2.20 decision 9).
//
// onProgress, when non-nil, is called after each formatting batch
// completes with SectionsProcessed/SectionsTotal set (every other Progress
// field left zero) — the desktop binding uses this to report "Formatting
// section X of Y" while the call is in flight (spec 2.20.1). A nil value
// means no such reporting is wanted.
func (s *Service) PreviewImport(
	ctx context.Context, sessionID string, root fs.FS, sourceRoot, filePath string,
	onProgress func(Progress) error,
) (ImportPreview, error) {
	if err := validateImportTarget(sessionID, filePath); err != nil {
		return ImportPreview{}, err
	}
	if err := s.index.CheckMutationAllowed(); err != nil {
		return ImportPreview{}, err
	}

	candidate := makeImportCandidate(sourceRoot, filePath)
	displayPath := filePath

	existing, err := s.ingestedFiles.ListBySession(ctx, sessionID)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("ingest: listing previously ingested files: %w", err)
	}
	prev, hasPrev := existing[candidate.SourcePath]
	if hasPrev {
		displayPath = prev.Path
	}

	mtime, err := modTime(root, candidate.ReadPath)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("ingest: reading file metadata: %w", err)
	}

	if hasPrev && prev.MTimeUnixNano == mtime && prev.EmbeddingModel == domainllm.EmbeddingModel {
		skip, _, checkErr := s.shouldSkipUnchanged(ctx, sessionID, prev)
		if checkErr != nil {
			return ImportPreview{}, fmt.Errorf("ingest: %w", checkErr)
		}
		if skip {
			return ImportPreview{DisplayPath: displayPath, Skipped: true, SkippedReason: "unchanged since last import"}, nil
		}
	}

	raw, err := fs.ReadFile(root, candidate.ReadPath)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("ingest: reading file: %w", err)
	}
	original := string(raw)

	isMarkdown := strings.EqualFold(path.Ext(candidate.ReadPath), ".md")
	sections := splitIntoFormattingSections(original)
	batches := packFormattingBatches(sections)
	var onSectionFormatted func(processed, total int) error
	if onProgress != nil {
		onSectionFormatted = func(processed, total int) error {
			return onProgress(Progress{SectionsProcessed: processed, SectionsTotal: total})
		}
	}
	outcomes, err := s.formatBatches(ctx, sessionID, original, batches, isMarkdown, onSectionFormatted)
	if err != nil {
		return ImportPreview{}, err
	}
	formatted := reassembleFormattedContent(original, outcomes)
	failed := anyBatchFellBack(outcomes)

	var chunkCandidates []ChunkCandidate
	if isMarkdown {
		chunkCandidates = ChunkMarkdown(formatted)
	} else {
		chunkCandidates = ChunkText(formatted)
	}

	chunks := make([]ChunkPreview, len(chunkCandidates))
	for i, c := range chunkCandidates {
		charCount := runeLen(c.Content)
		chunks[i] = ChunkPreview{
			Heading:         c.Heading,
			Content:         c.Content,
			CharCount:       charCount,
			EstimatedTokens: charCount / 4,
		}
	}

	return ImportPreview{
		DisplayPath:       displayPath,
		OriginalContent:   original,
		FormattedContent:  formatted,
		FormattingFailed:  failed,
		FailedSections:    failedSections(outcomes),
		StructureWarnings: structureWarnings(outcomes),
		Stats:             computeFormattingStats(original, formatted),
		Chunks:            chunks,
	}, nil
}

// correctiveListInstruction is appended to the system prompt for the one
// bounded retry formatBatches allows when validateFormattingStructure
// reports a detected named list still collapsed into one chunk — never
// used on the first attempt, since most batches do not need it.
const correctiveListInstruction = " The list below still reads as one block after a previous rewrite attempt: " +
	"restate each item's shared heading or intro context inside its own sentence, with a full blank line " +
	"between every item, so each one is long and self-contained enough to stand as its own chunk."

// formattingPrompt builds formatBatch's system prompt: the extension-aware
// base prompt (decision 3), plus breadcrumb context (decision 4) when
// non-empty, plus the corrective list instruction when corrective is true
// (decision from the structural-validation follow-up — one retry only,
// see formatBatches).
func formattingPrompt(breadcrumb string, isMarkdown, corrective bool) string {
	prompt := noteFormattingPlainTextPrompt
	if isMarkdown {
		prompt = noteFormattingMarkdownPrompt
	}
	if breadcrumb != "" {
		prompt += " This excerpt is part of a larger note, under the section \"" + breadcrumb +
			"\" — use that only as context for restating shared meaning; never add it to, or repeat it verbatim inside, your output."
	}
	if corrective {
		prompt += correctiveListInstruction
	}
	return prompt
}

// chatFormat is formatBatch's underlying Chat round-trip: one non-streamed
// call with prompt as the system message. A Provider error or a blank
// response degrades to the original content (failed = true) instead of
// blocking the import over a transient formatting failure — mirroring
// HydeEnricher.Enrich's blank-response guard (2.5.1), but falling back
// instead of erroring.
func (s *Service) chatFormat(ctx context.Context, sessionID, prompt, content string) (formatted string, failed bool) {
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

// formatBatch rewrites one formatting batch's content into clean Markdown
// (isMarkdown) or well-structured plain text (!isMarkdown) — the same
// contract formatNote (pre-2.20.1) used for a whole document, now scoped
// to a single batch. breadcrumb, when non-empty, is folded into the
// system prompt as context only — never into the user message — so the
// model never has reason to echo it back inside the rewritten text it
// returns.
func (s *Service) formatBatch(ctx context.Context, sessionID, breadcrumb, content string, isMarkdown bool) (formatted string, failed bool) {
	return s.chatFormat(ctx, sessionID, formattingPrompt(breadcrumb, isMarkdown, false), content)
}

// formatBatchWithCorrection re-runs formatBatch's same batch with
// correctiveListInstruction added — formatBatches' one bounded retry for a
// batch whose first rewrite passed fidelity validation but still left a
// detected named list collapsed into one chunk (see
// validateFormattingStructure).
func (s *Service) formatBatchWithCorrection(ctx context.Context, sessionID, breadcrumb, content string, isMarkdown bool) (formatted string, failed bool) {
	return s.chatFormat(ctx, sessionID, formattingPrompt(breadcrumb, isMarkdown, true), content)
}

// formatBatches runs formatBatch over every batch in document order,
// isolating each batch's failure to itself (spec 2.20.1) instead of the
// whole-document fallback formatNote used to have. A batch whose Chat call
// succeeds still has its result run through validateFormattingFidelity;
// failing that deterministic check forces the same fallback-to-original
// behavior as a Provider error or a blank response, since neither can be
// trusted. A batch that passes fidelity validation but whose structure
// check (validateFormattingStructure) reports a still-collapsed named
// list gets exactly one corrective retry (formatBatchWithCorrection); if
// that retry does not pass fidelity validation and improve the structure
// verdict too, the batch keeps its first successful rewrite with
// Structure.Improved left false — accepted, not fallen back to, but
// flagged rather than reported as silent success (20-01 decision 6).
// onSectionFormatted, when non-nil, is called with each batch's 1-based
// position and the total batch count after that batch completes (whether
// or not a corrective retry ran) — PreviewImport's desktop binding uses
// this to report "Formatting section X of Y" progress; a nil value means
// no such reporting is wanted.
func (s *Service) formatBatches(
	ctx context.Context, sessionID, source string, batches []FormattingBatch, isMarkdown bool,
	onSectionFormatted func(processed, total int) error,
) ([]BatchOutcome, error) {
	outcomes := make([]BatchOutcome, len(batches))
	for i, batch := range batches {
		original := source[batch.Start:batch.End]
		breadcrumb := batch.breadcrumb()
		formatted, failed := s.formatBatch(ctx, sessionID, breadcrumb, original, isMarkdown)

		var validation BatchValidation
		var structure StructureCheck
		if !failed {
			validation = validateFormattingFidelity(original, formatted, batch.headings())
			if !validation.Passed {
				formatted = original
				failed = true
			}
		}

		if !failed {
			structure = validateFormattingStructure(original, formatted, isMarkdown)
			if !structure.Improved {
				retryFormatted, retryFailed := s.formatBatchWithCorrection(ctx, sessionID, breadcrumb, original, isMarkdown)
				if !retryFailed {
					if retryValidation := validateFormattingFidelity(original, retryFormatted, batch.headings()); retryValidation.Passed {
						if retryStructure := validateFormattingStructure(original, retryFormatted, isMarkdown); retryStructure.Improved {
							formatted, validation, structure = retryFormatted, retryValidation, retryStructure
						}
					}
				}
			}
		}

		outcomes[i] = BatchOutcome{
			Batch: batch, Original: original, Formatted: formatted, Fallback: failed,
			Validation: validation, Structure: structure,
		}

		if onSectionFormatted != nil {
			if err := onSectionFormatted(i+1, len(batches)); err != nil {
				return nil, err
			}
		}
	}
	return outcomes, nil
}

// anyBatchFellBack reports whether at least one batch's formatting
// degraded to its original content — ImportPreview.FormattingFailed's
// source of truth now that formatting runs per batch instead of once for
// the whole document.
func anyBatchFellBack(outcomes []BatchOutcome) bool {
	for _, o := range outcomes {
		if o.Fallback {
			return true
		}
	}
	return false
}

// defaultFailedSectionReason is used when a batch fell back without a
// recorded BatchValidation reason — a Provider error or timeout, which
// formatBatch degrades on without capturing the underlying error text.
const defaultFailedSectionReason = "the formatter did not return a usable rewrite (provider error, timeout, or empty response)"

// failedSections maps every fallen-back batch into a FailedSection, by
// heading and reason, for ImportPreview.FailedSections.
func failedSections(outcomes []BatchOutcome) []FailedSection {
	var out []FailedSection
	for _, o := range outcomes {
		if !o.Fallback {
			continue
		}
		reason := strings.Join(o.Validation.Reasons, "; ")
		if reason == "" {
			reason = defaultFailedSectionReason
		}
		out = append(out, FailedSection{Heading: o.Batch.breadcrumb(), Reason: reason})
	}
	return out
}

// structureWarnings maps every accepted-but-not-structurally-improved
// batch into a StructureWarning for ImportPreview.StructureWarnings —
// skipping any batch that fell back (already covered by failedSections).
func structureWarnings(outcomes []BatchOutcome) []StructureWarning {
	var out []StructureWarning
	for _, o := range outcomes {
		if o.Fallback || o.Structure.Improved {
			continue
		}
		out = append(out, StructureWarning{
			Heading: o.Batch.breadcrumb(),
			Message: "a list of distinct items in this section may still be grouped into one chunk after formatting",
		})
	}
	return out
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
