package ingest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildShadowItem_concept_usesFilesH1_whenPresent(t *testing.T) {
	// Given a file with an H1 heading
	content := "# My Concept\nBody text."
	chunks := ChunkMarkdown(content)

	// When building the shadow item
	_, concept, _ := BuildShadowItem("go/my-concept.md", content, chunks)

	// Then Concept is the H1 text
	assert.Equal(t, "My Concept", concept)
}

func TestBuildShadowItem_concept_fallsBackToBaseNameWithoutExtension_whenNoH1(t *testing.T) {
	// Given a file with no H1 at all (only prose, or a lower-level heading)
	content := "## Not An H1\nBody text."
	chunks := ChunkMarkdown(content)

	// When building the shadow item for go/my-notes.md
	_, concept, _ := BuildShadowItem("go/my-notes.md", content, chunks)

	// Then Concept falls back to the base name without extension
	assert.Equal(t, "my-notes", concept)
}

func TestBuildShadowItem_concept_ignoresH1InsideACodeFence(t *testing.T) {
	// Given a file whose only "# " line is inside a fenced code block
	content := "```\n# not a heading\n```\nBody text."
	chunks := ChunkMarkdown(content)

	// When building the shadow item
	_, concept, _ := BuildShadowItem("notes/example.md", content, chunks)

	// Then Concept falls back to the base name, since there is no real H1
	assert.Equal(t, "example", concept)
}

func TestBuildShadowItem_definition_isFirst300CharsOfLeadingChunk_truncatedAtWordBoundary(t *testing.T) {
	// Given a leading chunk whose content is well over 300 characters
	content := strings.Repeat("palavra ", 100) // 800 chars, space-separated words
	chunks := ChunkMarkdown(content)

	// When building the shadow item
	_, _, definition := BuildShadowItem("notes/long.md", content, chunks)

	// Then the definition is cut at (or before) 300 runes, on a word
	// boundary, with a trailing ellipsis, and is never longer than the source
	assert.True(t, strings.HasSuffix(definition, "…"))
	assert.LessOrEqual(t, runeLen(definition), 301) // 300 + the ellipsis rune
	assert.False(t, strings.HasSuffix(strings.TrimSuffix(definition, "…"), " "))
}

func TestBuildShadowItem_definition_isReturnedAsIs_whenUnder300Chars(t *testing.T) {
	// Given a leading chunk shorter than the preview budget
	content := "# Título\nUma nota curta."
	chunks := ChunkMarkdown(content)

	// When building the shadow item
	_, _, definition := BuildShadowItem("notes/short.md", content, chunks)

	// Then the full body is kept, with no truncation ellipsis, and the
	// redundant "# Título" heading line is not repeated inside it
	assert.Equal(t, "Uma nota curta.", definition)
}

func TestBuildShadowItem_definition_fallsBackToAPlaceholder_whenThereAreNoChunks(t *testing.T) {
	// Given no chunks at all (an empty file)
	_, _, definition := BuildShadowItem("notes/empty.md", "", nil)

	// Then the definition falls back to a fixed, non-blank placeholder
	// rather than "" — knowledge.Item.Validate rejects a blank Definition,
	// and the shadow Item must stay valid to save and to edit back to
	// valid from the Explorer
	assert.Equal(t, emptyFileDefinition, definition)
}

func TestBuildShadowItem_topic_usesFirstLevelDirectoryUnderRoot(t *testing.T) {
	// Given a file nested two levels under the picked root
	content := "# Título\nBody."
	chunks := ChunkMarkdown(content)

	// When building the shadow item for go/advanced/generics.md
	topic, _, _ := BuildShadowItem("go/advanced/generics.md", content, chunks)

	// Then Topic is only the first-level directory, "go"
	assert.Equal(t, "go", topic)
}

func TestBuildShadowItem_topic_fallsBackToH1_whenFileIsDirectlyAtRoot(t *testing.T) {
	// Given a file with no containing subdirectory, but with an H1
	content := "# Root Note\nBody."
	chunks := ChunkMarkdown(content)

	// When building the shadow item for root-note.md (no "/")
	topic, _, _ := BuildShadowItem("root-note.md", content, chunks)

	// Then Topic falls back to the H1
	assert.Equal(t, "Root Note", topic)
}

func TestTruncateAtWordBoundary_keepsTextUnchanged_whenExactlyAtMaxChars(t *testing.T) {
	// Given text exactly as long as the budget
	text := exactlyNChars(10)

	// When truncating at that same budget
	result := truncateAtWordBoundary(text, 10)

	// Then it is returned unchanged, with no ellipsis added
	assert.Equal(t, text, result)
}

func TestBuildShadowItem_topic_fallsBackToBaseName_whenAtRootAndNoH1(t *testing.T) {
	// Given a file at the root with no H1 either
	content := "Plain prose, no heading at all."
	chunks := ChunkMarkdown(content)

	// When building the shadow item for plain-notes.md
	topic, _, _ := BuildShadowItem("plain-notes.md", content, chunks)

	// Then Topic falls back to the base name without extension
	assert.Equal(t, "plain-notes", topic)
}
