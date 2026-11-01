package exercise

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

func fullGenerationContext() generationContext {
	return generationContext{
		Topic:   "Distributed Caching",
		Goal:    "Ace the interview",
		Summary: "We covered eviction.",
		Profile: domainexercise.GenerationProfile{Area: "backend", ExperienceLevel: "advanced", Language: "pt"},
		Notes:   []domainexercise.NoteExcerpt{{Source: "cache.md", Content: "LRU evicts the oldest."}},
	}
}

func TestBuildGenerationPrompt_sendsASystemAndAUserMessage(t *testing.T) {
	// When the prompt is built
	messages := buildGenerationPrompt(fullGenerationContext(), domainexercise.DifficultyAdvanced)

	// Then there is one system message followed by one user message
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
}

func TestBuildGenerationPrompt_forbidsRevealingASolution(t *testing.T) {
	// When the prompt is built
	system := buildGenerationPrompt(fullGenerationContext(), domainexercise.DifficultyAdvanced)[0].Content

	// Then it tells the model not to give the answer away
	assert.Contains(t, system, "Do not include a solution, an architecture, or the names of the components")
}

func TestBuildGenerationPrompt_namesTheAssistantLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{"pt", "Brazilian Portuguese"},
		{"en", "English"},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			// Given a user whose assistant language is tt.language
			gc := fullGenerationContext()
			gc.Profile.Language = tt.language

			// When the prompt is built
			system := buildGenerationPrompt(gc, domainexercise.DifficultyBeginner)[0].Content

			// Then the model is told to write in that language
			assert.Contains(t, system, "Write every text in "+tt.want)
		})
	}
}

func TestBuildGenerationPrompt_statesTheDifficulty(t *testing.T) {
	// When the prompt is built for an advanced challenge
	system := buildGenerationPrompt(fullGenerationContext(), domainexercise.DifficultyAdvanced)[0].Content

	// Then the difficulty is named
	assert.Contains(t, system, "Difficulty: ADVANCED")
}

func TestBuildGenerationPrompt_describesTheJSONOutput(t *testing.T) {
	// When the prompt is built
	system := buildGenerationPrompt(fullGenerationContext(), domainexercise.DifficultyAdvanced)[0].Content

	// Then every field of the expected JSON is listed, with the criteria limits
	for _, field := range []string{
		`"title"`, `"scenario"`, `"functionalRequirements"`, `"nonFunctionalRequirements"`,
		`"constraints"`, `"objectives"`, `"criteria"`, `"name"`, `"description"`, `"weight"`, `"mandatory"`, `"levels"`,
	} {
		assert.Contains(t, system, field)
	}
	assert.Contains(t, system, "1 to 6 criteria")
	assert.Contains(t, system, "exactly 5 levels")
	assert.Contains(t, system, "only the JSON")
}

func TestBuildGenerationPrompt_putsTheContextInOrder(t *testing.T) {
	// When the prompt is built from a full context
	user := buildGenerationPrompt(fullGenerationContext(), domainexercise.DifficultyAdvanced)[1].Content

	// Then topic, goal, summary, profile and notes appear in that order
	positions := []int{
		strings.Index(user, "Distributed Caching"),
		strings.Index(user, "Ace the interview"),
		strings.Index(user, "We covered eviction."),
		strings.Index(user, "backend"),
		strings.Index(user, "LRU evicts the oldest."),
	}
	for i, position := range positions {
		require.GreaterOrEqual(t, position, 0, "piece %d missing", i)
		if i > 0 {
			assert.Greater(t, position, positions[i-1], "piece %d out of order", i)
		}
	}
	assert.Contains(t, user, "advanced")
	assert.Contains(t, user, "cache.md")
}

func TestBuildGenerationPrompt_leavesOutMissingPieces(t *testing.T) {
	// Given a context with no goal, summary or notes
	gc := fullGenerationContext()
	gc.Goal, gc.Summary, gc.Notes = "", "", nil

	// When the prompt is built
	user := buildGenerationPrompt(gc, domainexercise.DifficultyAdvanced)[1].Content

	// Then their headings are absent
	assert.NotContains(t, user, "Goal")
	assert.NotContains(t, user, "Conversation summary")
	assert.NotContains(t, user, "Notes")
}

func TestBuildGenerationPrompt_usesTheLabTask(t *testing.T) {
	assert.Equal(t, domainllm.TaskArchitectureLab, generationTask)
}
