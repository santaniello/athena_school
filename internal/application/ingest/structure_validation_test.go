package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectNamedListPattern_threeOrMoreShortBulletLines_isDetected(t *testing.T) {
	content := "Políticas de evicção:\n" +
		"- LRU: remove o mais antigo.\n" +
		"- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n" +
		"- RR: remove aleatoriamente.\n"
	assert.True(t, detectNamedListPattern(content))
}

func TestDetectNamedListPattern_numberedShortItems_isDetected(t *testing.T) {
	content := "Steps:\n1. First step.\n2. Second step.\n3. Third step.\n"
	assert.True(t, detectNamedListPattern(content))
}

func TestDetectNamedListPattern_fewerThanThreeItems_isNotDetected(t *testing.T) {
	content := "- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n"
	assert.False(t, detectNamedListPattern(content))
}

func TestDetectNamedListPattern_longBulletLines_areNotCountedAsNamedListItems(t *testing.T) {
	// Given bullet lines that are really full paragraphs, not short named items
	content := "- " + exactlyNChars(200) + "\n- " + exactlyNChars(200) + "\n- " + exactlyNChars(200) + "\n"
	assert.False(t, detectNamedListPattern(content))
}

func TestDetectNamedListPattern_prose_isNotDetected(t *testing.T) {
	content := "This is an ordinary paragraph of prose with no bullet or numbered lines at all, just sentences."
	assert.False(t, detectNamedListPattern(content))
}

func TestValidateFormattingStructure_namedListStillOneChunkAfterFormatting_isStillCollapsed(t *testing.T) {
	// Given the exact real-world shape diagnosed in 03-01-chunking-by-paragraph.md:
	// a tight bulleted list with no blank line between items, so it chunks as one
	// block both before and after a "formatting" pass that didn't actually split it
	original := "### Políticas de Evicção\n" +
		"- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n- RR: remove aleatoriamente.\n"
	formatted := original // the formatter's rewrite changed nothing structural

	check := validateFormattingStructure(original, formatted, true)

	assert.True(t, check.NamedListDetected)
	assert.True(t, check.StillCollapsed)
	assert.False(t, check.Improved)
	assert.Equal(t, 1, check.ChunksAfter)
}

func TestValidateFormattingStructure_namedListSplitIntoSeparateChunks_isImproved(t *testing.T) {
	// Given a formatted rewrite that gave each item enough restated context
	// to clear minChunkChars and land in its own chunk
	original := "### Políticas de Evicção\n" +
		"- LRU: remove o mais antigo.\n- LFU: remove o menos usado.\n" +
		"- FIFO: remove o primeiro que entrou.\n- RR: remove aleatoriamente.\n"
	formatted := "### Políticas de Evicção\n\n" +
		"LRU (Least Recently Used) é uma política de evicção que remove o item " + exactlyNChars(180) + ".\n\n" +
		"LFU (Least Frequently Used) é uma política de evicção que remove o item " + exactlyNChars(180) + ".\n\n" +
		"FIFO (First In First Out) é uma política de evicção que remove o item " + exactlyNChars(180) + ".\n\n" +
		"RR (Random Replacement) é uma política de evicção que remove o item " + exactlyNChars(180) + "."

	check := validateFormattingStructure(original, formatted, true)

	assert.True(t, check.NamedListDetected)
	assert.False(t, check.StillCollapsed)
	assert.True(t, check.Improved)
	assert.Greater(t, check.ChunksAfter, 1)
}

func TestValidateFormattingStructure_noNamedListInOriginal_isAlwaysImproved(t *testing.T) {
	// Given ordinary prose with no named-list shape at all
	original := "Just a normal paragraph with no bullet points, well over the chunk floor to stand alone."
	formatted := original

	check := validateFormattingStructure(original, formatted, true)

	assert.False(t, check.NamedListDetected)
	assert.False(t, check.StillCollapsed)
	assert.True(t, check.Improved)
}
