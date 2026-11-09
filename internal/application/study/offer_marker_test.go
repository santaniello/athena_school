package study

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	domainprofile "github.com/santaniello/athena/internal/domain/profile"
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

func TestStripExerciseOfferMarker(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantText  string
		wantFound bool
	}{
		{"no marker", "Just a normal answer.", "Just a normal answer.", false},
		{"marker at the end", "Sure, let's practice. " + exerciseOfferMarker, "Sure, let's practice.", true},
		{"marker on its own line", "Sure.\n\n" + exerciseOfferMarker + "\n", "Sure.", true},
		{"marker in the middle", "Sure " + exerciseOfferMarker + " go", "Sure  go", true},
		{"repeated marker", exerciseOfferMarker + "Hi" + exerciseOfferMarker, "Hi", true},
		{"partial marker is kept", "Sure [[EXERCISE", "Sure [[EXERCISE", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When stripping the marker
			text, found := stripExerciseOfferMarker(tt.content)

			// Then the text is clean and the detection is reported
			assert.Equal(t, tt.wantText, text)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func collectFiltered(chunks []string) (emitted string, found bool) {
	var out strings.Builder
	filter := &offerMarkerFilter{}
	for _, chunk := range chunks {
		out.WriteString(filter.write(chunk))
	}
	out.WriteString(filter.flush())
	_, found = stripExerciseOfferMarker(strings.Join(chunks, ""))
	return out.String(), found
}

func TestOfferMarkerFilter_neverEmitsTheMarker_howeverTheStreamIsSplit(t *testing.T) {
	full := "Sure, let's practice." + exerciseOfferMarker
	// Every possible split of the stream into two chunks
	for i := 0; i <= len(full); i++ {
		// When the stream is filtered
		emitted, found := collectFiltered([]string{full[:i], full[i:]})

		// Then the marker never reaches the user and is detected
		assert.Equal(t, "Sure, let's practice.", strings.TrimSpace(emitted), "split at %d", i)
		assert.True(t, found, "split at %d", i)
	}
}

func TestOfferMarkerFilter_emitsAHeldPartialMarker_whenTheStreamEndsWithoutCompletingIt(t *testing.T) {
	// Given a stream that ends in what looked like the start of the marker
	// When it is filtered
	emitted, found := collectFiltered([]string{"Use arrays like [[", "EXERCISE"})

	// Then the text is delivered intact and no offer is detected
	assert.Equal(t, "Use arrays like [[EXERCISE", emitted)
	assert.False(t, found)
}

func TestOfferMarkerFilter_passesPlainTextThroughImmediately(t *testing.T) {
	// Given plain text with no marker prefix at its end
	filter := &offerMarkerFilter{}

	// When a chunk is written
	// Then it is emitted at once, not held back
	assert.Equal(t, "Hello world", filter.write("Hello world"))
}
