package study

import "strings"

// exerciseOfferMarker is the fixed token the model ends a reply with when the
// user explicitly asked for an architecture exercise. The backend reads it,
// removes it from the stored and displayed text, and creates the offer. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md.
const exerciseOfferMarker = "[[ATHENA_EXERCISE_OFFER]]"

// exerciseOfferInstruction is the study-prompt rule that makes the model emit
// the marker. It is a pure string so the prompt test pins it.
const exerciseOfferInstruction = "\n- If, and only if, the user explicitly asks you for an exercise or to practice this topic " +
	"(for example \"give me an exercise\" or \"I want to practice this\"), answer in one or two short sentences " +
	"and end the reply with the exact text " + exerciseOfferMarker + " on its own line. " +
	"Never propose an exercise on your own, and never write that text in any other case."

// stripExerciseOfferMarker removes every marker from content and reports
// whether there was one. Content without a marker is returned untouched.
func stripExerciseOfferMarker(content string) (string, bool) {
	if !strings.Contains(content, exerciseOfferMarker) {
		return content, false
	}
	return strings.TrimSpace(strings.ReplaceAll(content, exerciseOfferMarker, "")), true
}

// offerMarkerFilter sits in the chunk stream shown to the user. It drops
// complete markers and holds back a trailing partial marker, so the user
// never sees even a fragment of it. A fragment that never completes is
// released by flush.
type offerMarkerFilter struct {
	pending string
	found   bool
}

// write takes the next chunk and returns the text that is safe to show now.
func (f *offerMarkerFilter) write(chunk string) string {
	f.pending += chunk
	if strings.Contains(f.pending, exerciseOfferMarker) {
		f.found = true
		f.pending = strings.ReplaceAll(f.pending, exerciseOfferMarker, "")
	}
	held := partialMarkerSuffixLength(f.pending)
	safe := f.pending[:len(f.pending)-held]
	f.pending = f.pending[len(f.pending)-held:]
	return safe
}

// flush returns what is still held when the stream ends.
func (f *offerMarkerFilter) flush() string {
	rest := f.pending
	f.pending = ""
	return rest
}

// partialMarkerSuffixLength is the length of the longest suffix of text that
// is a proper prefix of the marker.
func partialMarkerSuffixLength(text string) int {
	longest := min(len(text), len(exerciseOfferMarker)-1)
	for n := longest; n > 0; n-- {
		if strings.HasPrefix(exerciseOfferMarker, text[len(text)-n:]) {
			return n
		}
	}
	return 0
}
