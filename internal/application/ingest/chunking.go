// Package ingest implements the notes-import pipeline: parsing, chunking,
// embedding and persisting personal notes as knowledge.Chunk/knowledge.Item
// records. See
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md.
package ingest

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// maxChunkChars/minChunkChars bound a chunk's size in characters (runes),
// approximating tokens at ~4 chars/token (conservative for Portuguese,
// which runs closer to 3.5 chars/token). A budget, not a hard ceiling: a
// merge that keeps short content from being dropped is allowed to push a
// chunk over maxChunkChars.
const (
	maxChunkChars = 2000
	minChunkChars = 200
)

// ChunkCandidate is one heading- or paragraph-scoped slice of raw source
// text produced by ChunkMarkdown or ChunkText, before it is assembled into
// a persisted knowledge.Chunk (which also needs Source/Topic/ItemID/etc.,
// filled in by the caller).
type ChunkCandidate struct {
	Heading string
	Content string
	// Start and End are byte offsets into the original source string, at
	// UTF-8 boundaries, spanning the raw text this candidate was built
	// from. Unlike Content — which normalizes paragraph joins to "\n\n"
	// and is what gets embedded — source[Start:End] is the literal,
	// unmodified slice: the source's original spacing survives here even
	// where Content does not. Chunks stay ordered and non-overlapping.
	Start, End int
}

// ChunkMarkdown splits raw markdown source into heading-scoped chunks
// bounded by maxChunkChars/minChunkChars. Heading boundaries are flat: any
// level 1-3 heading starts a new section regardless of nesting, so a
// chunk's Heading is always the text of the single heading that opened its
// section, never a breadcrumb. Text before the first heading (or the
// entire document, when it has no H1-H3 at all) falls back to the
// paragraph-splitting behaviour of ChunkText, with Heading = "".
func ChunkMarkdown(source string) []ChunkCandidate {
	rawSections := findHeadingSections(source)
	if rawSections == nil {
		return ChunkText(source)
	}

	var pieces []ChunkCandidate
	for _, section := range rawSections {
		pieces = append(pieces, packParagraphs(section.Content, section.Heading, section.Start)...)
	}
	return mergeUndersized(pieces)
}

// ChunkText splits raw plain-text source on paragraph (blank-line)
// boundaries under the same budget as ChunkMarkdown, with Heading always
// "". Used for .txt files and as ChunkMarkdown's fallback for markdown
// with no H1-H3 heading.
func ChunkText(source string) []ChunkCandidate {
	return mergeUndersized(packParagraphs(source, "", 0))
}

// findHeadingSections walks source's goldmark AST for level 1-3 headings
// (ignoring anything that looks like a heading inside a fenced code block,
// since goldmark parses the real block structure rather than scanning
// lines) and slices the raw markdown between them. Each section's Content
// includes its own heading line, so a later merge never loses the losing
// side's heading text. Returns nil when source has no H1-H3 heading at all.
func findHeadingSections(source string) []ChunkCandidate {
	src := []byte(source)
	doc := goldmark.DefaultParser().Parse(text.NewReader(src))

	type boundary struct {
		start int
		title string
	}
	var boundaries []boundary
	walkErr := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		if heading.Level > 3 {
			return ast.WalkSkipChildren, nil
		}
		lines := heading.Lines()
		if lines.Len() == 0 {
			return ast.WalkSkipChildren, nil
		}
		seg := lines.At(0)
		lineStart := seg.Start
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}
		boundaries = append(boundaries, boundary{
			start: lineStart,
			title: strings.TrimSpace(string(seg.Value(src))),
		})
		return ast.WalkSkipChildren, nil
	})

	// The callback above never returns an error, so walkErr is always nil
	// in practice; treated the same as "no heading found" rather than
	// ignored, in case that ever changes.
	if walkErr != nil || len(boundaries) == 0 {
		return nil
	}

	var sections []ChunkCandidate
	// No "boundaries[0].start > 0" guard needed here: when the document's
	// first heading opens at byte 0, or the text before it is whitespace
	// only, raw is empty or trims to empty and relEnd > relStart alone
	// already excludes it.
	if raw := string(src[:boundaries[0].start]); raw != "" {
		relStart, relEnd := trimSpaceOffsets(raw)
		if relEnd > relStart {
			sections = append(sections, ChunkCandidate{
				Heading: "",
				Content: raw[relStart:relEnd],
				Start:   relStart,
				End:     relEnd,
			})
		}
	}
	for i, b := range boundaries {
		end := len(src)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].start
		}
		raw := string(src[b.start:end])
		relStart, relEnd := trimSpaceOffsets(raw)
		sections = append(sections, ChunkCandidate{
			Heading: b.title,
			Content: raw[relStart:relEnd],
			Start:   b.start + relStart,
			End:     b.start + relEnd,
		})
	}
	return sections
}

// trimSpaceOffsets returns the start and end byte offsets, within s, of the
// substring strings.TrimSpace(s) would produce — the same trimming rule
// (unicode.IsSpace on leading/trailing runes), without discarding the
// position the trimmed text sits at.
func trimSpaceOffsets(s string) (start, end int) {
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !unicode.IsSpace(r) {
			break
		}
		start += size
	}
	end = len(s)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(s[start:end])
		if !unicode.IsSpace(r) {
			break
		}
		end -= size
	}
	return start, end
}

var paragraphSeparator = regexp.MustCompile(`\n\s*\n`)

// paragraphSpan is one paragraph produced by splitParagraphs: its trimmed
// text plus the byte offsets, within the content string passed to
// splitParagraphs, that it was trimmed down from.
type paragraphSpan struct {
	Text       string
	Start, End int
}

// splitParagraphs breaks content on blank-line boundaries, returning each
// paragraph's trimmed text alongside its offsets within content.
func splitParagraphs(content string) []paragraphSpan {
	trimStart, trimEnd := trimSpaceOffsets(content)
	trimmed := content[trimStart:trimEnd]
	if trimmed == "" {
		return nil
	}

	var spans []paragraphSpan
	addSegment := func(rawStart, rawEnd int) {
		raw := trimmed[rawStart:rawEnd]
		relStart, relEnd := trimSpaceOffsets(raw)
		if relEnd <= relStart {
			return
		}
		spans = append(spans, paragraphSpan{
			Text:  raw[relStart:relEnd],
			Start: trimStart + rawStart + relStart,
			End:   trimStart + rawStart + relEnd,
		})
	}

	segStart := 0
	for _, sep := range paragraphSeparator.FindAllStringIndex(trimmed, -1) {
		addSegment(segStart, sep[0])
		segStart = sep[1]
	}
	addSegment(segStart, len(trimmed))
	return spans
}

// packParagraphs greedily packs content's paragraphs into pieces no larger
// than maxChunkChars, never splitting a paragraph itself — a single
// paragraph over budget is kept whole as its own oversized piece. Every
// piece carries heading unchanged. baseOffset is content's own byte offset
// within the ultimate document source, so each piece's Start/End — the
// first paragraph's start and the last paragraph's end, per document order
// — come out as absolute offsets into that source.
func packParagraphs(content, heading string, baseOffset int) []ChunkCandidate {
	paragraphs := splitParagraphs(content)
	if len(paragraphs) == 0 {
		return nil
	}

	var pieces []ChunkCandidate
	var current []string
	var currentStart, currentEnd int
	currentLen := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		pieces = append(pieces, ChunkCandidate{
			Heading: heading,
			Content: strings.Join(current, "\n\n"),
			Start:   baseOffset + currentStart,
			End:     baseOffset + currentEnd,
		})
		current = nil
		currentLen = 0
	}

	for _, p := range paragraphs {
		pLen := runeLen(p.Text)
		// current's own length guards this the same way flush does (a
		// flush on an empty current is a no-op below), so the length
		// check alone decides whether to close the piece being built.
		if currentLen+2+pLen > maxChunkChars { // +2 for the "\n\n" separator
			flush()
		}
		if len(current) == 0 {
			currentStart = p.Start
		}
		current = append(current, p.Text)
		currentEnd = p.End
		if len(current) == 1 {
			currentLen = pLen
		} else {
			currentLen += 2 + pLen
		}
	}
	flush()
	return pieces
}

// mergeUndersized folds every piece under minChunkChars into a neighbour
// so the store never fills with junk vectors, without ever dropping
// content. An undersized piece merges forward into the next one; the last
// piece, having no next, merges backward into its predecessor instead (or
// is kept as-is if it is the only piece). Either direction, the merged
// piece's Heading is whichever side dominates by content volume — the
// other side's heading text is not lost, it simply stays inside Content.
// A merge is allowed to push a piece over maxChunkChars; no re-split
// follows.
func mergeUndersized(pieces []ChunkCandidate) []ChunkCandidate {
	if len(pieces) == 0 {
		return pieces
	}
	working := append([]ChunkCandidate(nil), pieces...)
	result := make([]ChunkCandidate, 0, len(working))

	for i := range working {
		cur := working[i]
		isLast := i == len(working)-1
		undersized := runeLen(cur.Content) < minChunkChars

		switch {
		case !undersized:
			result = append(result, cur)
		case !isLast:
			working[i+1] = mergeCandidates(cur, working[i+1])
		case len(result) == 0:
			result = append(result, cur)
		default:
			result[len(result)-1] = mergeCandidates(result[len(result)-1], cur)
		}
	}
	return result
}

// mergeCandidates concatenates first and second in that (document) order,
// keeping whichever original Heading belongs to the larger side by
// content volume. The merged piece's Start/End are first's Start and
// second's End — first and second are always contiguous, non-overlapping
// document order, so this never widens the span beyond what the two
// pieces originally covered.
func mergeCandidates(first, second ChunkCandidate) ChunkCandidate {
	heading := first.Heading
	if runeLen(second.Content) > runeLen(first.Content) {
		heading = second.Heading
	}
	return ChunkCandidate{
		Heading: heading,
		Content: strings.TrimSpace(first.Content + "\n\n" + second.Content),
		Start:   first.Start,
		End:     second.End,
	}
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}
