package ingest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkMarkdown_singleSectionUnderBudget_isKeptWhole(t *testing.T) {
	// Given a small document with one heading and a body well under the budget
	source := "## Título\nCorpo curto o suficiente para não precisar de merge nem split, com bastante texto de enchimento para passar de duzentos caracteres no total do conteúdo desta seção única de teste."

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then it becomes exactly one chunk carrying that heading
	require.Len(t, chunks, 1)
	assert.Equal(t, "Título", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "Corpo curto")
}

func TestChunkMarkdown_headingBoundariesAreFlat_neverABreadcrumb(t *testing.T) {
	// Given a document with a level-1 heading containing nested level-2/3 headings
	source := "# Física\n" + repeatFiller("intro física ", 20) +
		"\n\n## Cinemática\n" + repeatFiller("corpo cinemática ", 20) +
		"\n\n### Aceleração\n" + repeatFiller("corpo aceleração ", 20)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then each chunk's Heading is only its own single title, never a breadcrumb
	require.Len(t, chunks, 3)
	assert.Equal(t, "Física", chunks[0].Heading)
	assert.Equal(t, "Cinemática", chunks[1].Heading)
	assert.Equal(t, "Aceleração", chunks[2].Heading)
	for _, c := range chunks {
		assert.NotContains(t, c.Heading, ">")
	}
}

func TestChunkMarkdown_ignoresHashInsideFencedCodeBlock(t *testing.T) {
	// Given a document whose only real heading is followed by a code fence
	// containing a line that looks like a markdown heading
	source := "## Real Heading\n" + repeatFiller("body text ", 30) +
		"\n\n```\n# not a heading\n```\n"

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the fenced "#" line never starts a new section
	require.Len(t, chunks, 1)
	assert.Equal(t, "Real Heading", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "# not a heading")
}

func TestChunkMarkdown_levelFourHeading_doesNotStartNewSection(t *testing.T) {
	// Given a level-3 heading followed by a level-4 sub-heading
	source := "### Sub Three\n" + repeatFiller("body ", 30) +
		"\n\n#### Deep Detail\n" + repeatFiller("deep body ", 30)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the level-4 heading stays embedded in the level-3 section
	require.Len(t, chunks, 1)
	assert.Equal(t, "Sub Three", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "#### Deep Detail")
}

func TestChunkMarkdown_sectionExactlyAtMaxChunkChars_isKeptWhole(t *testing.T) {
	// Given a section whose content is exactly maxChunkChars long
	body := exactlyNChars(maxChunkChars - len("# Heading\n"))
	source := "# Heading\n" + body

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then it is kept as a single, unsplit chunk
	require.Len(t, chunks, 1)
}

func TestChunkMarkdown_sectionOverMaxChunkChars_splitsOnParagraphBoundary(t *testing.T) {
	// Given a section whose content is well over maxChunkChars, made of
	// several distinct paragraphs
	var paragraphs []string
	for i := 0; i < 6; i++ {
		paragraphs = append(paragraphs, exactlyNChars(500))
	}
	source := "# Heading\n" + strings.Join(paragraphs, "\n\n")

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then it is split into more than one chunk, none exceeding the budget,
	// and every chunk keeps the parent heading
	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		assert.LessOrEqual(t, runeLen(c.Content), maxChunkChars)
		assert.Equal(t, "Heading", c.Heading)
	}
}

func TestChunkMarkdown_sectionUnderMinChunkChars_mergesForwardIntoNext_dominantHeadingWins(t *testing.T) {
	// Given a tiny stub section immediately followed by a much larger one
	source := "## Stub\ntiny\n\n## Main\n" + exactlyNChars(1000)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then they merge into one chunk, and the larger section's heading dominates
	require.Len(t, chunks, 1)
	assert.Equal(t, "Main", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "Stub")
	assert.Contains(t, chunks[0].Content, "tiny")
}

func TestChunkMarkdown_shortFinalSection_mergesBackwardsIntoPredecessor(t *testing.T) {
	// Given a normal-sized section followed by a tiny final stub
	source := "## Main\n" + exactlyNChars(1000) + "\n\n## Stub\ntiny"

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the stub is merged backwards into its predecessor, not dropped
	require.Len(t, chunks, 1)
	assert.Equal(t, "Main", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "Stub")
	assert.Contains(t, chunks[0].Content, "tiny")
}

func TestChunkMarkdown_onlySectionAndUndersized_isKeptAsIs(t *testing.T) {
	// Given a document with exactly one, tiny section
	source := "## Lonely\ntiny body"

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then it is kept as-is rather than dropped or merged with nothing
	require.Len(t, chunks, 1)
	assert.Equal(t, "Lonely", chunks[0].Heading)
	assert.Contains(t, chunks[0].Content, "tiny body")
}

func TestChunkMarkdown_mergeThatExceedsMaxChunkChars_isAcceptedNotResplit(t *testing.T) {
	// Given a full-budget section immediately followed by a tiny stub that
	// would push the merged chunk over maxChunkChars
	source := "## Main\n" + exactlyNChars(maxChunkChars-10) + "\n\n## Stub\ntiny stub text here"

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the merge still happens and the overshoot is accepted, not re-split
	require.Len(t, chunks, 1)
	assert.Greater(t, runeLen(chunks[0].Content), maxChunkChars)
}

func TestChunkMarkdown_noHeadingsAtAll_fallsBackToParagraphSplitting(t *testing.T) {
	// Given a document using only plain prose, no H1-H3 heading
	source := exactlyNChars(500) + "\n\n" + exactlyNChars(500)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then it still produces chunks (via the .txt-style fallback), each with an empty heading
	require.NotEmpty(t, chunks)
	for _, c := range chunks {
		assert.Empty(t, c.Heading)
	}
}

func TestChunkMarkdown_textBeforeFirstHeading_becomesLeadingChunkWithEmptyHeading(t *testing.T) {
	// Given front-matter-like prose before the first heading
	source := exactlyNChars(300) + "\n\n## First Heading\n" + exactlyNChars(300)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the leading chunk has an empty heading and the second carries the real one
	require.Len(t, chunks, 2)
	assert.Empty(t, chunks[0].Heading)
	assert.Equal(t, "First Heading", chunks[1].Heading)
}

func TestChunkMarkdown_whitespaceOnlyBeforeFirstHeading_producesNoLeadingChunk(t *testing.T) {
	// Given only blank lines before the first heading — no real lead text
	source := "\n\n\n## First Heading\n" + exactlyNChars(300)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then no bogus empty leading chunk is produced
	require.Len(t, chunks, 1)
	assert.Equal(t, "First Heading", chunks[0].Heading)
}

func TestFindHeadingSections_offsetsAreTrimmedSlicesOfTheirOwnRawSpan(t *testing.T) {
	// Given whitespace-only lead text (no real content before the first
	// heading) and an indented heading (so its raw span has leading
	// whitespace of its own to trim)
	source := "\n\n  ## Heading\n" + exactlyNChars(300)

	// When finding heading sections directly
	sections := findHeadingSections(source)

	// Then the whitespace-only lead produces no section at all, and the
	// one real section's Start/End slice exactly its own trimmed raw span
	require.Len(t, sections, 1)
	assert.Equal(t, sections[0].Content, source[sections[0].Start:sections[0].End])
	assert.True(t, strings.HasPrefix(sections[0].Content, "## Heading\n"), "got: %q", sections[0].Content)
}

func TestChunkMarkdown_indentedHeading_offsetsStillSliceTheOriginalSource(t *testing.T) {
	// Given an ATX heading indented by leading spaces (valid CommonMark,
	// up to 3 spaces), so the heading section's raw slice has non-zero
	// leading whitespace to trim before the offsets are computed
	source := "  ## Heading\n" + exactlyNChars(300)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the offsets still slice out exactly the trimmed section — not
	// shifted the wrong way by the leading indent
	require.Len(t, chunks, 1)
	assert.Equal(t, chunks[0].Content, source[chunks[0].Start:chunks[0].End])
	assert.True(t, strings.HasPrefix(chunks[0].Content, "## Heading\n"), "got: %q", chunks[0].Content)
}

func TestChunkMarkdown_producesNoOverlap_betweenAdjacentChunks(t *testing.T) {
	// Given a document with two clearly distinct, budget-sized sections
	source := "## One\n" + strings.Repeat("aaaa ", 100) + "\n\n## Two\n" + strings.Repeat("bbbb ", 100)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the "aaaa" filler never leaks into the "Two" chunk and vice versa
	require.Len(t, chunks, 2)
	assert.NotContains(t, chunks[1].Content, "aaaa")
	assert.NotContains(t, chunks[0].Content, "bbbb")
}

func TestChunkMarkdown_sectionContent_includesItsOwnHeadingLineVerbatim(t *testing.T) {
	// Given a document with two headed sections
	source := "## One\n" + exactlyNChars(300) + "\n\n## Two\n" + exactlyNChars(300)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then each chunk's raw Content starts with its own "## Title" line —
	// proving the section boundary lands at the true start of the heading's
	// source line, not partway into it
	require.Len(t, chunks, 2)
	assert.True(t, strings.HasPrefix(chunks[0].Content, "## One\n"), "got: %q", chunks[0].Content)
	assert.True(t, strings.HasPrefix(chunks[1].Content, "## Two\n"), "got: %q", chunks[1].Content)
}

func TestPackParagraphs_combinedParagraphsExactlyAtMaxChunkChars_packIntoOnePiece(t *testing.T) {
	// Given two paragraphs whose combined length, plus the "\n\n" separator,
	// is exactly maxChunkChars
	para1 := exactlyNChars(1000)
	para2 := exactlyNChars(maxChunkChars - 1000 - 2)
	content := para1 + "\n\n" + para2

	// When packing them
	pieces := packParagraphs(content, "H", 0)

	// Then both paragraphs stay packed into a single piece
	require.Len(t, pieces, 1)
	assert.Equal(t, maxChunkChars, runeLen(pieces[0].Content))
}

func TestPackParagraphs_combinedParagraphsOneOverMaxChunkChars_splitsIntoTwoPieces(t *testing.T) {
	// Given the same two paragraphs, now one character over the budget
	para1 := exactlyNChars(1000)
	para2 := exactlyNChars(maxChunkChars - 1000 - 2 + 1)
	content := para1 + "\n\n" + para2

	// When packing them
	pieces := packParagraphs(content, "H", 0)

	// Then they no longer fit together and split into two pieces
	require.Len(t, pieces, 2)
}

func TestChunkMarkdown_sectionExactlyAtMinChunkChars_isNotMerged(t *testing.T) {
	// Given a first section whose content is exactly minChunkChars long,
	// followed by a normal section
	first := "## Small\n" + exactlyNChars(minChunkChars-len("## Small\n"))
	second := "## Big\n" + exactlyNChars(1000)
	source := first + "\n\n" + second

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the first section is kept on its own — exactly at the minimum is
	// not "under" it
	require.Len(t, chunks, 2)
	assert.Equal(t, "Small", chunks[0].Heading)
	assert.Equal(t, "Big", chunks[1].Heading)
}

func TestChunkMarkdown_mergeTieBreak_keepsFirstSectionsHeading_whenVolumesAreEqual(t *testing.T) {
	// Given two tiny sections with equal-length headings ("Aa"/"Bb") and
	// identical bodies, so both sides of the merge have exactly equal
	// content length
	body := exactlyNChars(50)
	source := "## Aa\n" + body + "\n\n## Bb\n" + body

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then on an exact tie, the earlier (first) section's heading wins
	require.Len(t, chunks, 1)
	assert.Equal(t, "Aa", chunks[0].Heading)
}

func TestChunkText_splitsLongProseOnParagraphBoundaries_underBudget(t *testing.T) {
	// Given plain text made of several 500-char paragraphs, well over budget together
	var paragraphs []string
	for i := 0; i < 6; i++ {
		paragraphs = append(paragraphs, exactlyNChars(500))
	}
	source := strings.Join(paragraphs, "\n\n")

	// When chunking it as plain text
	chunks := ChunkText(source)

	// Then every chunk stays within budget and carries no heading
	require.Greater(t, len(chunks), 1)
	for _, c := range chunks {
		assert.LessOrEqual(t, runeLen(c.Content), maxChunkChars)
		assert.Empty(t, c.Heading)
	}
}

func TestChunkText_singleShortParagraph_isKeptAsOneChunk(t *testing.T) {
	// Given a single short paragraph
	source := "Uma nota curta e direta."

	// When chunking it as plain text
	chunks := ChunkText(source)

	// Then it becomes exactly one chunk with an empty heading
	require.Len(t, chunks, 1)
	assert.Empty(t, chunks[0].Heading)
	assert.Equal(t, source, chunks[0].Content)
}

func TestChunkMarkdown_headingSection_offsetsSliceTheOriginalSourceVerbatim(t *testing.T) {
	// Given a single-paragraph heading section (no internal "\n\n" join, so
	// Content is not itself normalized)
	source := "## Título\nCorpo curto o suficiente para não precisar de merge nem split, com bastante texto de enchimento para passar de duzentos caracteres no total do conteúdo desta seção única de teste."

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then Start/End slice the original source into exactly Content
	require.Len(t, chunks, 1)
	require.GreaterOrEqual(t, chunks[0].Start, 0)
	require.LessOrEqual(t, chunks[0].End, len(source))
	assert.Equal(t, chunks[0].Content, source[chunks[0].Start:chunks[0].End])
}

func TestChunkMarkdown_packedParagraphs_offsetsPreserveOriginalSpacing_contentNormalizesIt(t *testing.T) {
	// Given a section with two paragraphs joined by irregular whitespace
	// ("\n \n", not a plain "\n\n"), well under budget so both pack into
	// one piece
	source := "## Heading\nPrimeiro parágrafo com texto suficiente para não ser descartado por estar vazio.\n \nSegundo parágrafo, também com texto suficiente, mas separado por espaço extra na linha em branco."

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the raw slice reproduces the original irregular separator...
	require.Len(t, chunks, 1)
	raw := source[chunks[0].Start:chunks[0].End]
	assert.Contains(t, raw, "\n \n")
	// ...while Content normalizes it to a plain blank line
	assert.NotContains(t, chunks[0].Content, "\n \n")
	assert.Contains(t, chunks[0].Content, "\n\n")
	// and the raw slice, once its own separator is normalized the same
	// way, matches Content exactly (same text, only the separator differs)
	assert.Equal(t, chunks[0].Content, strings.ReplaceAll(raw, "\n \n", "\n\n"))
}

func TestChunkMarkdown_mergeForward_offsetsSpanFromStubStartToMainEnd(t *testing.T) {
	// Given a tiny stub section immediately followed by a much larger one
	source := "## Stub\ntiny\n\n## Main\n" + exactlyNChars(1000)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the merged chunk's offsets span from the stub's own start (the
	// document's start here) to the main section's end (the document's end)
	require.Len(t, chunks, 1)
	assert.Equal(t, 0, chunks[0].Start)
	assert.Equal(t, len(source), chunks[0].End)
}

func TestChunkMarkdown_mergeBackward_offsetsSpanFromMainStartToStubEnd(t *testing.T) {
	// Given a normal-sized section followed by a tiny final stub
	source := "## Main\n" + exactlyNChars(1000) + "\n\n## Stub\ntiny"

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then the merged chunk's offsets span the whole document, from Main's
	// start to Stub's end
	require.Len(t, chunks, 1)
	assert.Equal(t, 0, chunks[0].Start)
	assert.Equal(t, len(source), chunks[0].End)
}

func TestChunkMarkdown_twoSections_offsetsAreOrderedAndNonOverlapping(t *testing.T) {
	// Given two clearly distinct, budget-sized sections
	source := "## One\n" + strings.Repeat("aaaa ", 100) + "\n\n## Two\n" + strings.Repeat("bbbb ", 100)

	// When chunking it
	chunks := ChunkMarkdown(source)

	// Then each chunk's offsets slice exactly its own heading's text out of
	// the source, in increasing, non-overlapping order
	require.Len(t, chunks, 2)
	assert.LessOrEqual(t, chunks[0].End, chunks[1].Start)
	assert.True(t, strings.HasPrefix(source[chunks[0].Start:chunks[0].End], "## One\n"))
	assert.True(t, strings.HasPrefix(source[chunks[1].Start:chunks[1].End], "## Two\n"))
	assert.NotContains(t, source[chunks[0].Start:chunks[0].End], "bbbb")
	assert.NotContains(t, source[chunks[1].Start:chunks[1].End], "aaaa")
}

func TestChunkText_offsetsSliceTheOriginalSourceForEachParagraph(t *testing.T) {
	// Given plain text made of two paragraphs too large to pack together
	// (combined over maxChunkChars), so each becomes its own chunk
	first := exactlyNChars(1200)
	second := exactlyNChars(1200)
	source := first + "\n\n" + second

	// When chunking it as plain text
	chunks := ChunkText(source)

	// Then each chunk's offsets slice out exactly its own paragraph
	require.Len(t, chunks, 2)
	assert.Equal(t, first, source[chunks[0].Start:chunks[0].End])
	assert.Equal(t, second, source[chunks[1].Start:chunks[1].End])
}

func TestPackParagraphs_singleParagraphAloneOverMaxChunkChars_isKeptWholeAsItsOwnPiece(t *testing.T) {
	// Given a single paragraph whose own length already exceeds
	// maxChunkChars, with nothing packed before it (current is empty on
	// the very first iteration)
	content := exactlyNChars(maxChunkChars + 500)

	// When packing it
	pieces := packParagraphs(content, "H", 0)

	// Then it is kept whole as its own oversized piece, not dropped or
	// wrongly flushed against an empty pack
	require.Len(t, pieces, 1)
	assert.Equal(t, content, pieces[0].Content)
	assert.Equal(t, 0, pieces[0].Start)
	assert.Equal(t, len(content), pieces[0].End)
}

func TestMergeUndersized_singlePieceList_keepsItAsIs_evenWhenUndersized(t *testing.T) {
	// Given a single undersized piece and nothing else in the list
	piece := ChunkCandidate{Heading: "Lonely", Content: "tiny", Start: 3, End: 7}

	// When merging
	result := mergeUndersized([]ChunkCandidate{piece})

	// Then it is returned unchanged: there is no neighbour to merge into,
	// forward or backward
	require.Len(t, result, 1)
	assert.Equal(t, piece, result[0])
}

func TestSplitParagraphs_laterParagraphWithItsOwnLeadingIndent_offsetsPointPastTheIndent(t *testing.T) {
	// Given a second paragraph with leading spaces of its own, beyond what
	// the blank-line separator already consumes — so its span's Start
	// depends on trimStart, rawStart, and relStart all being added
	// together, not any two of them alone
	content := "Primeiro parágrafo aqui.\n\n  Segundo parágrafo indentado aqui."

	// When splitting it into paragraphs directly
	spans := splitParagraphs(content)

	// Then the second span's offsets point exactly past its own indent
	require.Len(t, spans, 2)
	assert.Equal(t, "Segundo parágrafo indentado aqui.", spans[1].Text)
	assert.Equal(t, spans[1].Text, content[spans[1].Start:spans[1].End])
}

func TestSplitParagraphs_paragraphMadeSolelyOfNonASCIIWhitespace_isSkippedNotKeptAsEmptySpan(t *testing.T) {
	// Given two real paragraphs separated by a "paragraph" that is only a
	// non-breaking space (U+00A0): unicode.IsSpace treats it as
	// whitespace, but the paragraph separator regex's \s class is
	// ASCII-only, so it does not get absorbed into either surrounding
	// blank-line run and briefly looks like its own paragraph
	content := "Primeiro parágrafo.\n\n \n\nSegundo parágrafo."

	// When splitting it into paragraphs directly
	spans := splitParagraphs(content)

	// Then the whitespace-only middle span is skipped outright — exactly
	// two real paragraphs come out, not three (with an empty one between)
	require.Len(t, spans, 2)
	assert.Equal(t, "Primeiro parágrafo.", spans[0].Text)
	assert.Equal(t, "Segundo parágrafo.", spans[1].Text)
}

func TestChunkText_paragraphMadeSolelyOfNonASCIIWhitespace_isSkippedNotKeptAsEmptyParagraph(t *testing.T) {
	// Given the same shape via the public ChunkText entry point
	source := "Primeiro parágrafo, com texto suficiente para não ser descartado por vazio.\n\n \n\nSegundo parágrafo, também com texto suficiente para não ser descartado por vazio."

	// When chunking it as plain text
	chunks := ChunkText(source)

	// Then the whitespace-only middle "paragraph" is skipped rather than
	// kept as its own empty piece
	for _, c := range chunks {
		assert.NotEmpty(t, strings.TrimSpace(c.Content))
	}
	var joined string
	for _, c := range chunks {
		joined += c.Content
	}
	assert.NotContains(t, joined, " ")
}

func TestChunkText_leadingAndTrailingWhitespace_isExcludedFromOffsets(t *testing.T) {
	// Given a single short paragraph surrounded by blank lines
	source := "\n\n  Uma nota curta e direta.  \n\n"

	// When chunking it as plain text
	chunks := ChunkText(source)

	// Then the offsets point only at the trimmed text, not the surrounding
	// whitespace
	require.Len(t, chunks, 1)
	assert.Equal(t, "Uma nota curta e direta.", source[chunks[0].Start:chunks[0].End])
}

// repeatFiller repeats word n times, giving deterministic filler text well
// over minChunkChars without being a round multiple of it.
func repeatFiller(word string, n int) string {
	return strings.TrimSpace(strings.Repeat(word, n))
}

// exactlyNChars returns a string of exactly n runes, built from a
// repeating letters-only alphabet — no spaces or newlines — so it never
// accidentally contains a blank line, and TrimSpace never shortens it.
func exactlyNChars(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[i%len(alphabet)])
	}
	return b.String()
}
