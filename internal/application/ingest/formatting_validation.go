package ingest

import (
	"fmt"
	"regexp"
	"strings"
)

// minFormattingSizeRatio rejects a batch's rewrite when the formatted text
// shrinks below this fraction of the original's rune length — catching a
// truncated or garbled rewrite, distinct from (and in addition to) the
// plain "output is empty" guard formatNote already had.
const minFormattingSizeRatio = 0.5

// minWordsForLanguageSignal is the fewest tokenized words
// detectDominantLanguage needs before trusting its own signal — below
// this, a short or numeric-heavy batch returns "" (inconclusive) rather
// than risk a false block.
const minWordsForLanguageSignal = 8

// languageConfidenceMargin is how far ahead one language's stopword
// hit-rate must be over the other's before detectDominantLanguage commits
// to it. Anything closer is "" (inconclusive) — this detector is biased
// toward silence (a missed real translation) over wrongly flagging a
// legitimate same-language rewrite that happens to use different, heavier
// vocabulary.
const languageConfidenceMargin = 0.03

// ptStopwords/enStopwords are not a general-purpose language detector —
// just enough signal to catch the specific failure mode diagnosed against
// real notes: a formatter translating a Portuguese note into English (or
// the reverse) with no other sign anything went wrong.
var ptStopwords = map[string]bool{
	"de": true, "que": true, "não": true, "para": true, "uma": true,
	"os": true, "as": true, "do": true, "da": true, "em": true,
	"um": true, "se": true, "mais": true, "como": true, "mas": true,
	"também": true, "são": true, "foi": true, "ser": true, "está": true,
	"pelo": true, "pela": true, "isso": true, "muito": true, "quando": true,
	"entre": true, "sobre": true, "depois": true, "assim": true, "até": true,
}

var enStopwords = map[string]bool{
	"the": true, "and": true, "of": true, "to": true, "is": true,
	"in": true, "that": true, "it": true, "for": true, "with": true,
	"was": true, "are": true, "this": true, "but": true, "also": true,
	"be": true, "were": true, "when": true, "which": true, "an": true,
	"on": true, "by": true, "from": true, "about": true, "after": true,
	"between": true, "than": true, "into": true, "its": true, "these": true,
}

var wordPattern = regexp.MustCompile(`[\p{L}]+`)

// detectDominantLanguage returns "pt", "en", or "" when the text is too
// short or too evenly split between both stopword sets to call. See
// minWordsForLanguageSignal/languageConfidenceMargin for the thresholds
// that bias this toward never calling it, rather than calling it wrong.
func detectDominantLanguage(text string) string {
	words := wordPattern.FindAllString(strings.ToLower(text), -1)
	if len(words) < minWordsForLanguageSignal {
		return ""
	}

	var ptHits, enHits int
	for _, w := range words {
		if ptStopwords[w] {
			ptHits++
		}
		if enStopwords[w] {
			enHits++
		}
	}

	total := float64(len(words))
	ptRate := float64(ptHits) / total
	enRate := float64(enHits) / total

	switch {
	case ptRate-enRate > languageConfidenceMargin:
		return "pt"
	case enRate-ptRate > languageConfidenceMargin:
		return "en"
	default:
		return ""
	}
}

// preservedTokenPattern matches the facts a rewrite must never drop:
// URLs, and numbers (including decimals, thousands separators, and a
// trailing "%"). It intentionally does not try to parse version strings
// specially — "4.5.2" already matches as a numeric token via repeated
// "[.,]digits" groups, which is all presence-comparison needs. A URL
// match is greedy up to the next whitespace — trailingURLPunctuation below
// then strips prose punctuation (a trailing comma, period, …) that
// following the URL with a sentence, not the URL itself, would otherwise
// bake into the token.
var preservedTokenPattern = regexp.MustCompile(`https?://\S+|\d+(?:[.,]\d+)*%?`)

// trailingURLPunctuation is prose punctuation a URL can end a sentence
// with that is never itself part of the URL.
const trailingURLPunctuation = ",.;:!?)]"

// extractNumbersPercentsURLsVersions returns every preserved-fact token in
// text, for a presence comparison against the same extraction over the
// pre-formatting original — not an exact-order diff, just "is it still in
// there somewhere."
func extractNumbersPercentsURLsVersions(text string) []string {
	matches := preservedTokenPattern.FindAllString(text, -1)
	for i, m := range matches {
		if strings.HasPrefix(m, "http") {
			matches[i] = strings.TrimRight(m, trailingURLPunctuation)
		}
	}
	return matches
}

var codeBlockPattern = regexp.MustCompile("(?s)```.*?```")

// extractCodeBlocks returns every fenced code block in text, verbatim.
func extractCodeBlocks(text string) []string {
	return codeBlockPattern.FindAllString(text, -1)
}

// sizeRatio is formatted's rune length as a fraction of original's. An
// empty original (never happens in practice — a batch's content is never
// empty) is treated as already full-size, so it can never itself trigger
// minFormattingSizeRatio.
func sizeRatio(original, formatted string) float64 {
	originalLen := runeLen(original)
	if originalLen == 0 {
		return 1
	}
	return float64(runeLen(formatted)) / float64(originalLen)
}

// BatchValidation is formatBatches' deterministic (no-LLM) verdict on one
// batch's rewrite, computed before accepting it. A false Passed forces
// Fallback for that batch alone, the same all-or-nothing-per-batch
// semantics the plain empty-response guard already had.
type BatchValidation struct {
	Passed                        bool
	Reasons                       []string
	LanguageBefore, LanguageAfter string
}

// validateFormattingFidelity runs every deterministic check against a
// batch's original/formatted text before formatBatches accepts the
// rewrite. sectionHeadings are the batch's own sections' leaf headings
// (see FormattingBatch.headings) — checked for survival, case-
// insensitively, to catch a translated or otherwise dropped heading.
func validateFormattingFidelity(original, formatted string, sectionHeadings []string) BatchValidation {
	v := BatchValidation{Passed: true}

	if strings.TrimSpace(formatted) == "" {
		v.Passed = false
		v.Reasons = append(v.Reasons, "formatted text is empty")
		return v
	}

	if ratio := sizeRatio(original, formatted); ratio < minFormattingSizeRatio {
		v.Passed = false
		v.Reasons = append(v.Reasons, fmt.Sprintf("formatted text is %.0f%% the size of the original", ratio*100))
	}

	v.LanguageBefore = detectDominantLanguage(original)
	v.LanguageAfter = detectDominantLanguage(formatted)
	if v.LanguageBefore != "" && v.LanguageAfter != "" && v.LanguageBefore != v.LanguageAfter {
		v.Passed = false
		v.Reasons = append(v.Reasons, fmt.Sprintf("language changed: %s -> %s", v.LanguageBefore, v.LanguageAfter))
	}

	for _, token := range extractNumbersPercentsURLsVersions(original) {
		if !strings.Contains(formatted, token) {
			v.Passed = false
			v.Reasons = append(v.Reasons, fmt.Sprintf("token %q missing from the formatted text", token))
			break
		}
	}

	for _, block := range extractCodeBlocks(original) {
		if !strings.Contains(formatted, block) {
			v.Passed = false
			v.Reasons = append(v.Reasons, "a code block was altered or removed")
			break
		}
	}

	lowerFormatted := strings.ToLower(formatted)
	for _, heading := range sectionHeadings {
		if heading == "" {
			continue
		}
		if !strings.Contains(lowerFormatted, strings.ToLower(heading)) {
			v.Passed = false
			v.Reasons = append(v.Reasons, fmt.Sprintf("heading %q no longer appears in the formatted text", heading))
			break
		}
	}

	return v
}
