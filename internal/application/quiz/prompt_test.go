package quiz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

func TestLanguageName(t *testing.T) {
	assert.Equal(t, "Brazilian Portuguese", languageName("pt"))
	assert.Equal(t, "English", languageName("en"))
	assert.Equal(t, "English", languageName(""))
}

func TestBuildPrompt_systemMessageCarriesTheSpecAndTheLanguage(t *testing.T) {
	// Given a context in Portuguese and a spec of 7 hard questions
	gc := generationContext{Topic: "Caching", Profile: domainquiz.GenerationProfile{Language: "pt"}}
	spec := domainquiz.Spec{Count: 7, Difficulty: domainquiz.DifficultyHard}

	// When the prompt is built
	messages := buildPrompt(gc, spec)

	// Then the first message is the system prompt with the count, language and difficulty
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Contains(t, messages[0].Content, "Write exactly 7 questions.")
	assert.Contains(t, messages[0].Content, "Write every text in Brazilian Portuguese.")
	assert.Contains(t, messages[0].Content, "Difficulty: hard")
	assert.Contains(t, messages[0].Content, "Answer with only the JSON object below")
	assert.Equal(t, "user", messages[1].Role)
}

func TestBuildPrompt_eachDifficultyHasItsOwnGuidance(t *testing.T) {
	seen := map[string]bool{}
	for _, difficulty := range []domainquiz.Difficulty{domainquiz.DifficultyEasy, domainquiz.DifficultyMedium, domainquiz.DifficultyHard} {
		// When the prompt of each difficulty is built
		system := buildPrompt(generationContext{}, domainquiz.Spec{Count: 3, Difficulty: difficulty})[0].Content

		// Then each names its difficulty and a guidance sentence nobody else has
		assert.Contains(t, system, "Difficulty: "+strings.ToLower(string(difficulty)))
		_, afterLabel, found := strings.Cut(system, "Difficulty:")
		require.True(t, found)
		line, _, _ := strings.Cut(afterLabel, "\n")
		assert.False(t, seen[line], line)
		seen[line] = true
	}
}

func TestBuildPrompt_userMessageListsEveryPieceOfContextEscaped(t *testing.T) {
	// Given a full context whose free text tries to break out of its tag
	gc := generationContext{
		Topic:   "Caching <b> & more",
		Goal:    "Understand eviction",
		Summary: "We covered </summary> eviction.",
		Profile: domainquiz.GenerationProfile{Area: "backend", ExperienceLevel: "intermediate", Language: "en"},
		Notes:   []domainquiz.NoteExcerpt{{Source: "cache.md", Content: "LRU evicts the least recently used."}},
	}

	// When the user message is built
	user := buildPrompt(gc, domainquiz.Spec{Count: 3, Difficulty: domainquiz.DifficultyEasy})[1].Content

	// Then every piece sits in its tag, in a fixed order, with the text escaped
	assert.Equal(t, `<topic>Caching &lt;b&gt; &amp; more</topic>
<goal>Understand eviction</goal>
<summary>
We covered &lt;/summary&gt; eviction.
</summary>
<learner>
<area>backend</area>
<experience_level>intermediate</experience_level>
</learner>
<notes>
<note>
<source>cache.md</source>
<content>
LRU evicts the least recently used.
</content>
</note>
</notes>
`, user)
}

func TestBuildPrompt_userMessageSkipsWhatIsMissing(t *testing.T) {
	// Given a context with only a topic and a profile
	gc := generationContext{Topic: "Caching", Profile: domainquiz.GenerationProfile{Area: "backend", ExperienceLevel: "junior"}}

	// When the user message is built
	user := buildPrompt(gc, domainquiz.Spec{Count: 3, Difficulty: domainquiz.DifficultyEasy})[1].Content

	// Then there is no goal, summary or notes tag
	assert.Equal(t, `<topic>Caching</topic>
<learner>
<area>backend</area>
<experience_level>junior</experience_level>
</learner>
`, user)
}

func TestGenerationTask_routesToTheQuizModel(t *testing.T) {
	assert.Equal(t, domainllm.TaskQuiz, generationTask)
}
