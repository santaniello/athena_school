package ingest

import (
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// definitionPreviewChars bounds the shadow Item's Definition preview — a
// plain textual preview of the file's own words, never an LLM rewrite.
const definitionPreviewChars = 300

// BuildShadowItem derives the Topic/Concept/Definition for one imported
// file's shadow knowledge.Item, purely from already-parsed data — no LLM
// call, no I/O. relPath is the file's path relative to the picked import
// root, using "/" separators (an fs.FS path). chunks is the file's own
// ChunkMarkdown/ChunkText output; chunks[0] is its leading chunk.
func BuildShadowItem(relPath, content string, chunks []ChunkCandidate) (topic, concept, definition string) {
	h1, hasH1 := firstHeadingText(content, 1)
	concept = conceptFor(relPath, h1, hasH1)
	topic = topicFor(relPath, h1, hasH1)
	definition = definitionFor(chunks)
	return topic, concept, definition
}

// conceptFor is the file's H1, falling back to its base name without
// extension. Deliberately not the same fallback chain as topicFor: Concept
// must identify this specific file, not its category.
func conceptFor(relPath, h1 string, hasH1 bool) string {
	if hasH1 {
		return h1
	}
	return baseNameWithoutExt(relPath)
}

// topicFor is the first-level directory under the picked root, falling
// back to the file's H1, falling back to its base name — the same
// fallback chain used for the chunks this Item stands in for.
func topicFor(relPath, h1 string, hasH1 bool) string {
	if dir := firstPathSegment(relPath); dir != "" {
		return dir
	}
	if hasH1 {
		return h1
	}
	return baseNameWithoutExt(relPath)
}

// definitionFor is the first definitionPreviewChars characters of the
// file's leading chunk (chunks[0]), stripped of that chunk's own raw
// heading-marker line and truncated on a word boundary with a trailing
// "…" when it runs over budget.
func definitionFor(chunks []ChunkCandidate) string {
	if len(chunks) == 0 {
		return ""
	}
	return truncateAtWordBoundary(stripLeadingHeadingLine(chunks[0].Content), definitionPreviewChars)
}

var leadingHeadingLine = regexp.MustCompile(`^#{1,6}[ \t]+[^\n]*\n?`)

// stripLeadingHeadingLine removes a leading "# Title" ATX heading line —
// ChunkCandidate.Content includes its own heading line for merge fidelity,
// but a shadow Item's Definition is meant to read as plain prose.
func stripLeadingHeadingLine(content string) string {
	return leadingHeadingLine.ReplaceAllString(content, "")
}

// truncateAtWordBoundary returns s unchanged when it is at most maxChars
// runes long; otherwise it cuts at maxChars runes, backs up to the last
// whitespace boundary (so no word is cut mid-way), and appends "…".
func truncateAtWordBoundary(s string, maxChars int) string {
	trimmed := strings.TrimSpace(s)
	runes := []rune(trimmed)
	if len(runes) <= maxChars {
		return trimmed
	}
	cut := string(runes[:maxChars])
	if idx := strings.LastIndexAny(cut, " \t\n"); idx > 0 {
		cut = cut[:idx]
	}
	return strings.TrimSpace(cut) + "…"
}

// baseNameWithoutExt returns relPath's file name with its extension
// removed. relPath uses "/" separators (an fs.FS path), so this uses the
// "path" package rather than "path/filepath".
func baseNameWithoutExt(relPath string) string {
	base := path.Base(relPath)
	return strings.TrimSuffix(base, path.Ext(base))
}

// firstPathSegment returns the first path component of relPath, or "" when
// relPath has no directory component (the file sits at the import root).
func firstPathSegment(relPath string) string {
	relPath = strings.TrimPrefix(relPath, "/")
	if idx := strings.Index(relPath, "/"); idx > 0 {
		return relPath[:idx]
	}
	return ""
}

// firstHeadingText returns the title text of the first heading of the
// given level in source, or ("", false) when there is none. Uses the same
// goldmark AST walk as findHeadingSections, so a "#" inside a fenced code
// block is correctly never mistaken for a real heading.
func firstHeadingText(source string, level int) (string, bool) {
	src := []byte(source)
	doc := goldmark.DefaultParser().Parse(text.NewReader(src))

	var title string
	var found bool
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if found || !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		if heading.Level != level {
			return ast.WalkSkipChildren, nil
		}
		lines := heading.Lines()
		if lines.Len() == 0 {
			return ast.WalkSkipChildren, nil
		}
		seg := lines.At(0)
		title = strings.TrimSpace(string(seg.Value(src)))
		found = true
		return ast.WalkStop, nil
	})
	return title, found
}
