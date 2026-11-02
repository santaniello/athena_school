package exercise

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

func levelsJSON(count int) []string {
	levels := make([]string, count)
	for i := range levels {
		levels[i] = fmt.Sprintf("level %d", i)
	}
	return levels
}

func criterionJSON(name string, weight int) map[string]any {
	return map[string]any{
		"name": name, "description": "How well " + name + " is handled.",
		"weight": weight, "mandatory": false, "levels": levelsJSON(5),
	}
}

func validOutput() map[string]any {
	return map[string]any{
		"title":                     "Design a global cache",
		"scenario":                  "A shop serves 50k requests per second worldwide.",
		"functionalRequirements":    []string{"Serve product pages"},
		"nonFunctionalRequirements": []string{"p99 under 100 ms"},
		"constraints":               []string{"Three regions"},
		"objectives":                []string{"Justify the eviction policy"},
		"criteria":                  []any{criterionJSON("Cache invalidation", 10), criterionJSON("Hot keys", 5)},
	}
}

func encode(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func TestParseGeneration_buildsTheChallengeAndItsCriteria(t *testing.T) {
	// Given a valid model output
	content := encode(t, validOutput())

	// When it is parsed
	got, err := parseGeneration(content, "distributed caching", domainexercise.DifficultyAdvanced)

	// Then the challenge input carries the content, the difficulty and the AI origin
	require.NoError(t, err)
	assert.Equal(t, "Design a global cache", got.Challenge.Title)
	assert.Equal(t, "A shop serves 50k requests per second worldwide.", got.Challenge.Scenario)
	assert.Equal(t, []string{"Serve product pages"}, got.Challenge.FunctionalRequirements)
	assert.Equal(t, []string{"p99 under 100 ms"}, got.Challenge.NonFunctionalRequirements)
	assert.Equal(t, []string{"Three regions"}, got.Challenge.Constraints)
	assert.Equal(t, []string{"Justify the eviction policy"}, got.Challenge.Objectives)
	assert.Equal(t, domainexercise.DifficultyAdvanced, got.Challenge.Difficulty)
	assert.Equal(t, domainexercise.OriginAIGenerated, got.Challenge.Origin)

	// And the criteria carry name, description, weight, mandatory and the five levels
	require.Len(t, got.Criteria, 2)
	assert.Equal(t, "Cache invalidation", got.Criteria[0].Name)
	assert.Equal(t, "How well Cache invalidation is handled.", got.Criteria[0].Description)
	assert.Equal(t, 10, got.Criteria[0].Weight)
	assert.False(t, got.Criteria[0].Mandatory)
	assert.Equal(t, [domainexercise.LevelCount]string{"level 0", "level 1", "level 2", "level 3", "level 4"}, got.Criteria[0].Levels)
}

func TestParseGeneration_keepsTheMandatoryFlag(t *testing.T) {
	// Given a mandatory criterion
	output := validOutput()
	output["criteria"] = []any{map[string]any{
		"name": "Safety", "description": "d", "weight": 10, "mandatory": true, "levels": levelsJSON(5),
	}}

	// When it is parsed
	got, err := parseGeneration(encode(t, output), "caching", domainexercise.DifficultyBeginner)

	// Then the flag is kept
	require.NoError(t, err)
	assert.True(t, got.Criteria[0].Mandatory)
}

func TestParseGeneration_acceptsOutputInsideACodeFence(t *testing.T) {
	// Given a model that wrapped its JSON in a Markdown fence
	content := "```json\n" + encode(t, validOutput()) + "\n```"

	// When it is parsed
	_, err := parseGeneration(content, "caching", domainexercise.DifficultyBeginner)

	// Then it is accepted
	require.NoError(t, err)
}

func TestParseGeneration_derivesCriterionKeysFromNames(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		keys  []string
	}{
		{"slug with the c- prefix", []string{"Cache invalidation!"}, []string{"c-cache-invalidation"}},
		{"suffix for a repeated name", []string{"Hot keys", "Hot keys", "hot  KEYS"}, []string{"c-hot-keys", "c-hot-keys-2", "c-hot-keys-3"}},
		{"suffix that collides with another slug", []string{"Hot keys", "Hot keys", "Hot keys 2"}, []string{"c-hot-keys", "c-hot-keys-2", "c-hot-keys-2-2"}},
		{"accented letters are kept", []string{"Consistência"}, []string{"c-consistência"}},
		{"a name with no letters", []string{"!!!"}, []string{"c-criterion"}},
		{"a long name stays within the key limit", []string{strings.Repeat("a", 90)}, []string{"c-" + strings.Repeat("a", 40)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given criteria with those names
			output := validOutput()
			criteria := make([]any, len(tt.names))
			for i, name := range tt.names {
				criteria[i] = criterionJSON(name, 10)
			}
			output["criteria"] = criteria

			// When the output is parsed
			got, err := parseGeneration(encode(t, output), "caching", domainexercise.DifficultyBeginner)

			// Then the keys are derived by code
			require.NoError(t, err)
			keys := make([]string, len(got.Criteria))
			for i, criterion := range got.Criteria {
				keys[i] = criterion.Key
			}
			assert.Equal(t, tt.keys, keys)
		})
	}
}

func TestParseGeneration_rejectsInvalidOutput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(output map[string]any)
		raw    string
	}{
		{name: "text that is not JSON", raw: "Sure! Here is a challenge."},
		{name: "empty output", raw: ""},
		{name: "a JSON array", raw: "[]"},
		{name: "missing title", mutate: func(o map[string]any) { delete(o, "title") }},
		{name: "blank scenario", mutate: func(o map[string]any) { o["scenario"] = "  " }},
		{name: "no functional requirement", mutate: func(o map[string]any) { o["functionalRequirements"] = []string{} }},
		{name: "no objective", mutate: func(o map[string]any) { delete(o, "objectives") }},
		{name: "duplicated requirement", mutate: func(o map[string]any) { o["constraints"] = []string{"a", "a"} }},
		{name: "no criterion", mutate: func(o map[string]any) { o["criteria"] = []any{} }},
		{name: "seven criteria", mutate: func(o map[string]any) {
			criteria := make([]any, 7)
			for i := range criteria {
				criteria[i] = criterionJSON(fmt.Sprintf("Criterion %d", i), 5)
			}
			o["criteria"] = criteria
		}},
		{name: "four levels", mutate: func(o map[string]any) {
			c := criterionJSON("Hot keys", 5)
			c["levels"] = levelsJSON(4)
			o["criteria"] = []any{c}
		}},
		{name: "six levels", mutate: func(o map[string]any) {
			c := criterionJSON("Hot keys", 5)
			c["levels"] = levelsJSON(6)
			o["criteria"] = []any{c}
		}},
		{name: "blank level", mutate: func(o map[string]any) {
			c := criterionJSON("Hot keys", 5)
			c["levels"] = []string{"a", "b", "", "d", "e"}
			o["criteria"] = []any{c}
		}},
		{name: "zero weight", mutate: func(o map[string]any) { o["criteria"] = []any{criterionJSON("Hot keys", 0)} }},
		{name: "a weight that is not a whole number", raw: strings.Replace(encode(t, validOutput()), `"weight":10`, `"weight":10.5`, 1)},
		{name: "missing mandatory flag", mutate: func(o map[string]any) {
			c := criterionJSON("Hot keys", 5)
			delete(c, "mandatory")
			o["criteria"] = []any{c}
		}},
		{name: "null mandatory flag", mutate: func(o map[string]any) {
			c := criterionJSON("Hot keys", 5)
			c["mandatory"] = nil
			o["criteria"] = []any{c}
		}},
		{name: "blank criterion name", mutate: func(o map[string]any) { o["criteria"] = []any{criterionJSON(" ", 5)} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an output that breaks a rule
			content := tt.raw
			if tt.mutate != nil {
				output := validOutput()
				tt.mutate(output)
				content = encode(t, output)
			}

			// When it is parsed
			_, err := parseGeneration(content, "caching", domainexercise.DifficultyBeginner)

			// Then it is rejected as an invalid generation
			require.ErrorIs(t, err, ErrInvalidGeneration)
		})
	}
}

func TestParseGeneration_namesTheCriterionWithTheWrongLevelCount(t *testing.T) {
	// Given a second criterion with four levels
	output := validOutput()
	bad := criterionJSON("Hot keys", 5)
	bad["levels"] = levelsJSON(4)
	output["criteria"] = []any{criterionJSON("Cache invalidation", 10), bad}

	// When it is parsed
	_, err := parseGeneration(encode(t, output), "caching", domainexercise.DifficultyBeginner)

	// Then the error says which criterion and how many levels it had
	require.ErrorIs(t, err, ErrInvalidGeneration)
	assert.Contains(t, err.Error(), "criterion 2 has 4 levels, want 5")
}

func TestParseGeneration_namesTheCriterionWithoutAMandatoryFlag(t *testing.T) {
	// Given a second criterion with no mandatory flag
	output := validOutput()
	bad := criterionJSON("Hot keys", 5)
	delete(bad, "mandatory")
	output["criteria"] = []any{criterionJSON("Cache invalidation", 10), bad}

	// When it is parsed
	_, err := parseGeneration(encode(t, output), "caching", domainexercise.DifficultyBeginner)

	// Then the error says which criterion is missing the flag
	require.ErrorIs(t, err, ErrInvalidGeneration)
	assert.Contains(t, err.Error(), "criterion 2 has no mandatory flag")
}
