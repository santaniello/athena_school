// Package ingest implements the notes-import pipeline: parsing, chunking,
// embedding and persisting personal notes as knowledge.Chunk/knowledge.Item
// records. See
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md.
package ingest

import (
	"regexp"
	"strings"
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
		pieces = append(pieces, packParagraphs(section.Content, section.Heading)...)
	}
	return mergeUndersized(pieces)
}

// ChunkText splits raw plain-text source on paragraph (blank-line)
// boundaries under the same budget as ChunkMarkdown, with Heading always
// "". Used for .txt files and as ChunkMarkdown's fallback for markdown
// with no H1-H3 heading.
func ChunkText(source string) []ChunkCandidate {
	return mergeUndersized(packParagraphs(source, ""))
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
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
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

	if len(boundaries) == 0 {
		return nil
	}

	var sections []ChunkCandidate
	if lead := strings.TrimSpace(string(src[:boundaries[0].start])); lead != "" {
		sections = append(sections, ChunkCandidate{Heading: "", Content: lead})
	}
	for i, b := range boundaries {
		end := len(src)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].start
		}
		sections = append(sections, ChunkCandidate{
			Heading: b.title,
			Content: strings.TrimSpace(string(src[b.start:end])),
		})
	}
	return sections
}

var paragraphSeparator = regexp.MustCompile(`\n\s*\n`)

// splitParagraphs breaks content on blank-line boundaries.
func splitParagraphs(content string) []string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	raw := paragraphSeparator.Split(trimmed, -1)
	paragraphs := make([]string, 0, len(raw))
	for _, p := range raw {
		if p = strings.TrimSpace(p); p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	return paragraphs
}

// packParagraphs greedily packs content's paragraphs into pieces no larger
// than maxChunkChars, never splitting a paragraph itself — a single
// paragraph over budget is kept whole as its own oversized piece. Every
// piece carries heading unchanged.
func packParagraphs(content, heading string) []ChunkCandidate {
	paragraphs := splitParagraphs(content)
	if len(paragraphs) == 0 {
		return nil
	}

	var pieces []ChunkCandidate
	var current []string
	currentLen := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		pieces = append(pieces, ChunkCandidate{Heading: heading, Content: strings.Join(current, "\n\n")})
		current = nil
		currentLen = 0
	}

	for _, p := range paragraphs {
		pLen := runeLen(p)
		if len(current) > 0 && currentLen+2+pLen > maxChunkChars { // +2 for the "\n\n" separator
			flush()
		}
		current = append(current, p)
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
// content volume.
func mergeCandidates(first, second ChunkCandidate) ChunkCandidate {
	heading := first.Heading
	if runeLen(second.Content) > runeLen(first.Content) {
		heading = second.Heading
	}
	return ChunkCandidate{
		Heading: heading,
		Content: strings.TrimSpace(first.Content + "\n\n" + second.Content),
	}
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}
