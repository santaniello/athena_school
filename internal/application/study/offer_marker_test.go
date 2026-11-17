package study

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

func TestBuildSystemPrompt_instructsTheModelToEndWithTheOfferMarker_whenTheUserAsksForAnExercise(t *testing.T) {
	// Given any profile
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Caching", "Learn caching")

	// Then it names the marker and says it only follows an explicit request
	assert.Contains(t, prompt, exerciseOfferMarker)
	assert.Contains(t, prompt, "explicitly asks")
	assert.Contains(t, prompt, "Never propose an exercise on your own")
}

func TestBuildSystemPrompt_instructsTheModelToEndWithTheQuizMarker_whenTheUserAsksForAQuiz(t *testing.T) {
	// Given any profile
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Caching", "Learn caching")

	// Then it names the quiz marker and says it only follows an explicit request
	assert.Contains(t, prompt, quizOfferMarker)
	assert.Contains(t, prompt, "explicitly asks you for a quiz or a test")
	assert.Contains(t, prompt, "Never propose a quiz on your own")
}

func TestBuildSystemPrompt_instructsTheModelToEndWithTheMindMapMarker_whenTheUserAsksForAMindMap(t *testing.T) {
	// Given any profile
	profile := domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

	// When building the system prompt
	prompt := buildSystemPrompt(profile, "Caching", "Learn caching")

	// Then it names the mind map marker, says it only follows an explicit request
	// and says a quiz, test or exercise request is not one
	assert.Contains(t, prompt, mindMapOfferMarker)
	assert.Contains(t, prompt, "explicitly asks you for a mind map")
	assert.Contains(t, prompt, "A request for an exercise, a quiz or a test is not a request for a mind map")
	assert.Contains(t, prompt, "Never propose a mind map on your own")
}

func TestStripOfferMarkers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantText string
		wantKind domainstudy.OfferKind
	}{
		{"no marker", "Just a normal answer.", "Just a normal answer.", domainstudy.OfferKindNone},
		{"exercise marker at the end", "Sure, let's practice. " + exerciseOfferMarker, "Sure, let's practice.", domainstudy.OfferKindExercise},
		{"quiz marker at the end", "Sure, a quiz. " + quizOfferMarker, "Sure, a quiz.", domainstudy.OfferKindQuiz},
		{"mind map marker at the end", "Sure, a map. " + mindMapOfferMarker, "Sure, a map.", domainstudy.OfferKindMindMap},
		{"mind map marker on its own line", "Sure.\n\n" + mindMapOfferMarker + "\n", "Sure.", domainstudy.OfferKindMindMap},
		{"repeated mind map marker", mindMapOfferMarker + "Hi" + mindMapOfferMarker, "Hi", domainstudy.OfferKindMindMap},
		{"quiz and mind map markers, quiz wins", "Hi " + mindMapOfferMarker + quizOfferMarker, "Hi", domainstudy.OfferKindQuiz},
		{"exercise and mind map markers, exercise wins", "Hi " + mindMapOfferMarker + exerciseOfferMarker, "Hi", domainstudy.OfferKindExercise},
		{"all three markers, exercise wins", "Hi " + mindMapOfferMarker + quizOfferMarker + exerciseOfferMarker, "Hi", domainstudy.OfferKindExercise},
		{"partial mind map marker is kept", "Sure [[ATHENA_MINDMAP", "Sure [[ATHENA_MINDMAP", domainstudy.OfferKindNone},
		{"marker on its own line", "Sure.\n\n" + exerciseOfferMarker + "\n", "Sure.", domainstudy.OfferKindExercise},
		{"quiz marker on its own line", "Sure.\n\n" + quizOfferMarker + "\n", "Sure.", domainstudy.OfferKindQuiz},
		{"marker in the middle", "Sure " + exerciseOfferMarker + " go", "Sure  go", domainstudy.OfferKindExercise},
		{"repeated marker", exerciseOfferMarker + "Hi" + exerciseOfferMarker, "Hi", domainstudy.OfferKindExercise},
		{"repeated quiz marker", quizOfferMarker + "Hi" + quizOfferMarker, "Hi", domainstudy.OfferKindQuiz},
		{"both markers, exercise first", "Hi " + exerciseOfferMarker + quizOfferMarker, "Hi", domainstudy.OfferKindExercise},
		{"both markers, quiz first", "Hi " + quizOfferMarker + exerciseOfferMarker, "Hi", domainstudy.OfferKindExercise},
		{"partial marker is kept", "Sure [[EXERCISE", "Sure [[EXERCISE", domainstudy.OfferKindNone},
		{"partial quiz marker is kept", "Sure [[ATHENA_QUIZ", "Sure [[ATHENA_QUIZ", domainstudy.OfferKindNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When stripping the markers
			text, kind := stripOfferMarkers(tt.content)

			// Then the text is clean and the offer kind is reported
			assert.Equal(t, tt.wantText, text)
			assert.Equal(t, tt.wantKind, kind)
		})
	}
}

func collectFiltered(chunks []string) (emitted string, kind domainstudy.OfferKind) {
	var out strings.Builder
	filter := &offerMarkerFilter{}
	for _, chunk := range chunks {
		out.WriteString(filter.write(chunk))
	}
	out.WriteString(filter.flush())
	_, kind = stripOfferMarkers(strings.Join(chunks, ""))
	return out.String(), kind
}

func TestOfferMarkerFilter_neverEmitsAMarker_howeverTheStreamIsSplit(t *testing.T) {
	tests := []struct {
		name     string
		marker   string
		wantKind domainstudy.OfferKind
	}{
		{"exercise", exerciseOfferMarker, domainstudy.OfferKindExercise},
		{"quiz", quizOfferMarker, domainstudy.OfferKindQuiz},
		{"mind map", mindMapOfferMarker, domainstudy.OfferKindMindMap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			full := "Sure, let's go." + tt.marker
			// Every possible split of the stream into two chunks
			for i := 0; i <= len(full); i++ {
				// When the stream is filtered
				emitted, kind := collectFiltered([]string{full[:i], full[i:]})

				// Then the marker never reaches the user and is detected
				assert.Equal(t, "Sure, let's go.", strings.TrimSpace(emitted), "split at %d", i)
				assert.Equal(t, tt.wantKind, kind, "split at %d", i)
			}
		})
	}
}

func TestOfferMarkerFilter_neverEmitsEitherMarker_whenBothAppear(t *testing.T) {
	// Given a reply that carries both markers, split at every point
	full := "Hi." + quizOfferMarker + exerciseOfferMarker
	for i := 0; i <= len(full); i++ {
		// When the stream is filtered
		emitted, kind := collectFiltered([]string{full[:i], full[i:]})

		// Then neither marker reaches the user and the exercise wins
		assert.Equal(t, "Hi.", strings.TrimSpace(emitted), "split at %d", i)
		assert.Equal(t, domainstudy.OfferKindExercise, kind, "split at %d", i)
	}
}

func TestOfferMarkerFilter_emitsAHeldPartialMarker_whenTheStreamEndsWithoutCompletingIt(t *testing.T) {
	// Given a stream that ends in what looked like the start of the marker
	// When it is filtered
	emitted, kind := collectFiltered([]string{"Use arrays like [[", "EXERCISE"})

	// Then the text is delivered intact and no offer is detected
	assert.Equal(t, "Use arrays like [[EXERCISE", emitted)
	assert.Equal(t, domainstudy.OfferKindNone, kind)
}

func TestOfferMarkerFilter_emitsAHeldPartialQuizMarker_whenTheStreamEndsWithoutCompletingIt(t *testing.T) {
	// Given a stream that ends in what looked like the start of the quiz marker
	// When it is filtered
	emitted, kind := collectFiltered([]string{"Use arrays like [[ATHENA_", "QUIZ"})

	// Then the text is delivered intact and no offer is detected
	assert.Equal(t, "Use arrays like [[ATHENA_QUIZ", emitted)
	assert.Equal(t, domainstudy.OfferKindNone, kind)
}

func TestOfferMarkerFilter_neverEmitsAnyMarker_whenAllThreeAppear(t *testing.T) {
	// Given a reply that carries every marker, split at every point
	full := "Hi." + mindMapOfferMarker + quizOfferMarker + exerciseOfferMarker
	for i := 0; i <= len(full); i++ {
		// When the stream is filtered
		emitted, kind := collectFiltered([]string{full[:i], full[i:]})

		// Then no marker reaches the user and the exercise wins
		assert.Equal(t, "Hi.", strings.TrimSpace(emitted), "split at %d", i)
		assert.Equal(t, domainstudy.OfferKindExercise, kind, "split at %d", i)
	}
}

func TestOfferMarkerFilter_emitsAHeldPartialMindMapMarker_whenTheStreamEndsWithoutCompletingIt(t *testing.T) {
	// Given a stream that ends in what looked like the start of the mind map marker
	// When it is filtered
	emitted, kind := collectFiltered([]string{"Use arrays like [[ATHENA_", "MINDMAP"})

	// Then the text is delivered intact and no offer is detected
	assert.Equal(t, "Use arrays like [[ATHENA_MINDMAP", emitted)
	assert.Equal(t, domainstudy.OfferKindNone, kind)
}

func TestOfferMarkerFilter_passesPlainTextThroughImmediately(t *testing.T) {
	// Given plain text with no marker prefix at its end
	filter := &offerMarkerFilter{}

	// When a chunk is written
	// Then it is emitted at once, not held back
	assert.Equal(t, "Hello world", filter.write("Hello world"))
}
