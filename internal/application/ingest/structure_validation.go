package ingest

import (
	"regexp"
	"strings"
)

// namedListMinItems/namedListMaxItemChars define detectNamedListPattern's
// heuristic: a section mostly made of short, bullet- or numbered-line
// items — the exact shape (per-policy one-liners like "- FIFO: removes
// the oldest item") that collapses under minChunkChars even after the
// formatter restates context into each item, per the LRU/LFU/FIFO/RR case
// documented in 03-01-chunking-by-paragraph.md.
const (
	namedListMinItems     = 3
	namedListMaxItemChars = 80
)

var bulletLinePattern = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+\S`)

// detectNamedListPattern reports whether content reads as a list of
// several distinct, separately-definable items — a weak heuristic
// (line shape and length only, no semantic understanding), deliberately
// biased toward false negatives: missing a real named list costs nothing
// beyond skipping an optional corrective retry, while a false positive
// would waste one.
func detectNamedListPattern(content string) bool {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if !bulletLinePattern.MatchString(trimmed) {
			continue
		}
		if runeLen(strings.TrimSpace(trimmed)) > namedListMaxItemChars {
			continue
		}
		count++
	}
	return count >= namedListMinItems
}

// StructureCheck is formatBatches' informational verdict on whether a
// batch's chunking outcome looks structurally healthy. Unlike
// BatchValidation, a false Improved never forces Fallback by itself — the
// rewrite already passed fidelity validation, so it is still a legitimate
// improvement over the raw original; Improved=false only means a
// detected named list still collapsed into one chunk despite formatting,
// surfaced to the Review dialog instead of reported as silent success
// (see 20-01's decision 6 and Known risks).
type StructureCheck struct {
	Improved          bool
	NamedListDetected bool
	StillCollapsed    bool
	ChunksBefore      int
	ChunksAfter       int
}

// chunkContent runs the same chunker ConfirmImport will eventually use —
// never a separate "expected chunk count" oracle.
func chunkContent(content string, isMarkdown bool) []ChunkCandidate {
	if isMarkdown {
		return ChunkMarkdown(content)
	}
	return ChunkText(content)
}

// validateFormattingStructure chunks a batch's original and formatted
// content and compares the result against detectNamedListPattern's
// signal on the original. StillCollapsed is true only when a detected
// named list still produces exactly one chunk after formatting — the
// precise, previously-silent symptom this check exists to catch.
func validateFormattingStructure(original, formatted string, isMarkdown bool) StructureCheck {
	namedList := detectNamedListPattern(original)
	chunksBefore := len(chunkContent(original, isMarkdown))
	chunksAfter := len(chunkContent(formatted, isMarkdown))
	stillCollapsed := namedList && chunksAfter <= 1

	return StructureCheck{
		Improved:          !stillCollapsed,
		NamedListDetected: namedList,
		StillCollapsed:    stillCollapsed,
		ChunksBefore:      chunksBefore,
		ChunksAfter:       chunksAfter,
	}
}
