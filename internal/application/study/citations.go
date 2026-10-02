package study

import "regexp"

// citationMarker matches an inline citation, e.g. "[2]" — see
// specs/phases/phase-02-knowledge-engine/18-notebooklm-style-citations.md
// decision 1.
var citationMarker = regexp.MustCompile(`\[\d+\]`)

// stripCitationMarkers removes every [n] marker from content. It is applied
// only to prior assistant turns resent to the model — never to the
// persisted/displayed message — so the model never mistakes its own past
// citation markers for part of the conversation to continue citing from.
func stripCitationMarkers(content string) string {
	return citationMarker.ReplaceAllString(content, "")
}
