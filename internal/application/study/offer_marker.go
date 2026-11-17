package study

import (
	"fmt"
	"strings"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// exerciseOfferMarker is the fixed token the model ends a reply with when the
// user explicitly asked for an architecture exercise. The backend reads it,
// removes it from the stored and displayed text, and creates the offer. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md.
const exerciseOfferMarker = "[[ATHENA_EXERCISE_OFFER]]"

// exerciseOfferInstruction is the study-prompt rule that makes the model emit
// the marker. It is a pure string so the prompt test pins it.
var exerciseOfferInstruction = fmt.Sprintf(
	"\n- If, and only if, the user explicitly asks you for an exercise or to practice this topic "+
		"(for example \"give me an exercise\" or \"I want to practice this\"), answer in one or two short sentences "+
		"and end the reply with the exact text %s on its own line. "+
		"Never propose an exercise on your own, and never write that text in any other case.",
	exerciseOfferMarker,
)

// quizOfferMarker is the fixed token the model ends a reply with when the user
// explicitly asked for a quiz or a test about the session. It is handled like
// exerciseOfferMarker. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
const quizOfferMarker = "[[ATHENA_QUIZ_OFFER]]"

// quizOfferInstruction is the study-prompt rule that makes the model emit the
// quiz marker. It is a pure string so the prompt test pins it.
var quizOfferInstruction = fmt.Sprintf(
	"\n- If, and only if, the user explicitly asks you for a quiz or a test about this topic "+
		"(for example \"quiz me\" or \"give me a test\"), answer in one or two short sentences "+
		"and end the reply with the exact text %s on its own line. "+
		"Never propose a quiz on your own, and never write that text in any other case.",
	quizOfferMarker,
)

// mindMapOfferMarker is the fixed token the model ends a reply with when the
// user explicitly asked for a mind map of the session. It is handled like
// exerciseOfferMarker. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
const mindMapOfferMarker = "[[ATHENA_MINDMAP_OFFER]]"

// offerMarkers lists every marker, in the order of precedence when a reply
// carries more than one: the exercise offer wins, then the quiz.
var offerMarkers = []struct {
	marker string
	kind   domainstudy.OfferKind
}{
	{exerciseOfferMarker, domainstudy.OfferKindExercise},
	{quizOfferMarker, domainstudy.OfferKindQuiz},
	{mindMapOfferMarker, domainstudy.OfferKindMindMap},
}

// stripOfferMarkers removes every marker from content and reports the offer
// they ask for: the first of offerMarkers when several kinds are present,
// domainstudy.OfferKindNone when there is no marker. Content without a marker is returned untouched.
func stripOfferMarkers(content string) (string, domainstudy.OfferKind) {
	kind := domainstudy.OfferKindNone
	stripped := content
	for _, candidate := range offerMarkers {
		if !strings.Contains(stripped, candidate.marker) {
			continue
		}
		stripped = strings.ReplaceAll(stripped, candidate.marker, "")
		if kind == domainstudy.OfferKindNone {
			kind = candidate.kind
		}
	}
	if kind == domainstudy.OfferKindNone {
		return content, domainstudy.OfferKindNone
	}
	return strings.TrimSpace(stripped), kind
}

// offerMarkerFilter sits in the chunk stream shown to the user. It drops
// complete markers and holds back a trailing partial marker, so the user
// never sees even a fragment of one. A fragment that never completes is
// released by flush.
type offerMarkerFilter struct {
	pending string
}

// write takes the next chunk and returns the text that is safe to show now.
func (f *offerMarkerFilter) write(chunk string) string {
	f.pending += chunk
	for _, candidate := range offerMarkers {
		f.pending = strings.ReplaceAll(f.pending, candidate.marker, "")
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
// is a proper prefix of any marker.
func partialMarkerSuffixLength(text string) int {
	longest := 0
	for _, candidate := range offerMarkers {
		for n := len(candidate.marker) - 1; n > longest; n-- {
			if strings.HasSuffix(text, candidate.marker[:n]) {
				longest = n
				break
			}
		}
	}
	return longest
}
