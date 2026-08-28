package knowledge

import (
	"strings"
	"unicode"
)

// NormalizeConcept trims, lowercases Unicode, turns every run of
// non-letter/non-digit characters into one space, and collapses whitespace.
// It does not strip accents — "Padrão" stays "padrão", so an accented and
// unaccented spelling of the same word are never silently treated as an
// exact match; semantic detection may still catch that pair. It lives in
// domain rather than application (see 10-duplicate-detection.md) because
// internal/infrastructure/sqlite must call it too when comparing a
// candidate's concept against a persisted item's normalized_concept column
// — infrastructure may depend on domain, never on application. See
// specs/phases/phase-02-knowledge-engine/10-01-duplicate-detection-decisions.md
// Decision 1.
func NormalizeConcept(concept string) string {
	var b strings.Builder
	lastWasSeparator := true // collapses leading separators too
	for _, r := range strings.ToLower(concept) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastWasSeparator = false
			continue
		}
		if !lastWasSeparator {
			b.WriteRune(' ')
			lastWasSeparator = true
		}
	}
	return strings.TrimRight(b.String(), " ")
}
