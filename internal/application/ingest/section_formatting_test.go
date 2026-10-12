package ingest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitIntoFormattingSections_headingDocument_usesTheSameBreadcrumbsAsChunking(t *testing.T) {
	// Given a document with nested headings
	source := "# Física\nintro.\n\n## Cinemática\ncorpo.\n\n### Aceleração\ndetalhe."

	// When splitting it into formatting sections
	sections := splitIntoFormattingSections(source)

	// Then each section carries the same breadcrumb findHeadingSections
	// would produce for chunking
	require.Len(t, sections, 3)
	assert.Equal(t, "Física", sections[0].Breadcrumb)
	assert.Equal(t, "Física > Cinemática", sections[1].Breadcrumb)
	assert.Equal(t, "Física > Cinemática > Aceleração", sections[2].Breadcrumb)
}

func TestSplitIntoFormattingSections_noHeadingsAtAll_fallsBackToParagraphScopedSections(t *testing.T) {
	// Given a heading-less document (e.g. a .txt source) with two paragraphs
	source := exactlyNChars(300) + "\n\n" + exactlyNChars(300)

	// When splitting it into formatting sections
	sections := splitIntoFormattingSections(source)

	// Then it splits on paragraph boundaries, each with an empty breadcrumb
	require.Len(t, sections, 2)
	for _, s := range sections {
		assert.Empty(t, s.Breadcrumb)
	}
}

func TestPackFormattingBatches_smallSections_packIntoOneBatchUnderBudget(t *testing.T) {
	// Given several small sections well under maxFormattingChars combined
	sections := []FormattingSection{
		{Breadcrumb: "A", Content: "tiny a", Start: 0, End: 6},
		{Breadcrumb: "B", Content: "tiny b", Start: 6, End: 12},
	}

	// When packing them into batches
	batches := packFormattingBatches(sections)

	// Then they are grouped into a single batch
	require.Len(t, batches, 1)
	assert.Len(t, batches[0].Sections, 2)
}

func TestPackFormattingBatches_sectionOverBudgetAlone_becomesItsOwnBatch_neverSplit(t *testing.T) {
	// Given one section already over maxFormattingChars on its own
	oversized := exactlyNChars(maxFormattingChars + 500)
	sections := []FormattingSection{
		{Breadcrumb: "Big", Content: oversized, Start: 0, End: len(oversized)},
	}

	// When packing it into batches
	batches := packFormattingBatches(sections)

	// Then it is kept as its own batch, not dropped or split
	require.Len(t, batches, 1)
	require.Len(t, batches[0].Sections, 1)
	assert.Equal(t, oversized, batches[0].Sections[0].Content)
}

func TestPackFormattingBatches_sectionThatWouldOverflow_startsANewBatchInstead(t *testing.T) {
	// Given a first section near the budget and a second section that
	// would push the combined batch over it
	first := exactlyNChars(maxFormattingChars - 100)
	second := exactlyNChars(500)
	sections := []FormattingSection{
		{Breadcrumb: "First", Content: first, Start: 0, End: len(first)},
		{Breadcrumb: "Second", Content: second, Start: len(first), End: len(first) + len(second)},
	}

	// When packing them into batches
	batches := packFormattingBatches(sections)

	// Then the second section starts a new batch rather than overflowing the first
	require.Len(t, batches, 2)
	assert.Len(t, batches[0].Sections, 1)
	assert.Len(t, batches[1].Sections, 1)
	assert.Equal(t, "First", batches[0].Sections[0].Breadcrumb)
	assert.Equal(t, "Second", batches[1].Sections[0].Breadcrumb)
}

func TestReassembleFormattedContent_allBatchesFallback_isByteExactToTheOriginalSpan(t *testing.T) {
	// Given a document with two heading sections large enough on their own
	// to each force a separate batch, batched and then "formatted" with
	// every batch falling back to its own original content
	source := "## One\n" + exactlyNChars(maxFormattingChars) + "\n\n## Two\n" + exactlyNChars(maxFormattingChars)
	sections := splitIntoFormattingSections(source)
	batches := packFormattingBatches(sections)
	require.Len(t, batches, 2)

	outcomes := make([]BatchOutcome, len(batches))
	for i, batch := range batches {
		original := source[batch.Start:batch.End]
		outcomes[i] = BatchOutcome{Batch: batch, Original: original, Formatted: original, Fallback: true}
	}

	// When reassembling
	result := reassembleFormattedContent(source, outcomes)

	// Then the result is byte-exact to the original span across all batches,
	// including the original gap text between them
	expected := source[batches[0].Start:batches[len(batches)-1].End]
	assert.Equal(t, expected, result)
}

func TestReassembleFormattedContent_mixOfFallbackAndRewrite_preservesOriginalGapsBetweenBatches(t *testing.T) {
	// Given two batches, each large enough to force a separate batch,
	// where only the first falls back
	source := "## One\n" + exactlyNChars(maxFormattingChars) + "\n\n## Two\n" + exactlyNChars(maxFormattingChars)
	sections := splitIntoFormattingSections(source)
	batches := packFormattingBatches(sections)
	require.Len(t, batches, 2)

	outcomes := []BatchOutcome{
		{Batch: batches[0], Original: source[batches[0].Start:batches[0].End], Formatted: source[batches[0].Start:batches[0].End], Fallback: true},
		{Batch: batches[1], Original: source[batches[1].Start:batches[1].End], Formatted: "## Two (rewritten)", Fallback: false},
	}

	// When reassembling
	result := reassembleFormattedContent(source, outcomes)

	// Then the original gap between the two batches survives verbatim, and
	// the rewritten batch's text replaces its own original span
	gap := source[batches[0].End:batches[1].Start]
	expected := source[batches[0].Start:batches[0].End] + gap + "## Two (rewritten)"
	assert.Equal(t, expected, result)
	assert.True(t, strings.Contains(result, gap))
}

func TestFormattingBatch_breadcrumb_usesItsFirstSection(t *testing.T) {
	// Given a batch spanning two sections with different breadcrumbs
	batch := FormattingBatch{Sections: []FormattingSection{
		{Breadcrumb: "First > Section"},
		{Breadcrumb: "Second > Section"},
	}}

	// Then the batch's representative breadcrumb is its first section's
	assert.Equal(t, "First > Section", batch.breadcrumb())
}
