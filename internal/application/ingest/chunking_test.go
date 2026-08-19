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
	body := exactlyNChars(maxChunkChars - len("Heading\n"))
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
	pieces := packParagraphs(content, "H")

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
	pieces := packParagraphs(content, "H")

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
