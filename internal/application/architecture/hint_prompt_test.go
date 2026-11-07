package architecture

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

func testHintInput() domainexercise.HintInput {
	return domainexercise.HintInput{
		Challenge: domainexercise.Challenge{
			Title: "Design a URL shortener", Scenario: "A link service with 10M daily users.",
			FunctionalRequirements:    []string{"Shorten a URL"},
			NonFunctionalRequirements: []string{"p99 under 100 ms"},
			Constraints:               []string{"No new database"},
			Objectives:                []string{"Cut read load"},
		},
		Rubric: domainexercise.RubricVersion{Criteria: []domainexercise.RubricCriterion{
			{Name: "Scalability", Description: "Handles the load"},
		}},
		BoardXML: `<mxfile><diagram>a & b</diagram></mxfile>`,
		Previous: []domainexercise.Hint{
			{Position: 1, Content: "What happens if the cache is down?"},
			{Position: 2, Content: "Who owns the write path?"},
		},
	}
}

func TestBuildHintPrompt_hasASystemAndAUserMessage(t *testing.T) {
	// Given a hint input
	// When building the prompt
	messages := buildHintPrompt(testHintInput(), "en")

	// Then it is a system message followed by a user message
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
}

func TestBuildHintPrompt_systemPromptCarriesTheRules(t *testing.T) {
	// Given a hint input
	// When building the prompt for English
	system := buildHintPrompt(testHintInput(), "en")[0].Content

	// Then it forbids a solution, bounds the length and names the language
	assert.Contains(t, system, "Never give the solution")
	assert.Contains(t, system, "Do not name the component")
	assert.Contains(t, system, "Do not grade")
	assert.Contains(t, system, "at most 60 words")
	assert.Contains(t, system, "Write the hint in English.")
	assert.Contains(t, system, "untrusted data")
}

func TestBuildHintPrompt_usesTheAssistantLanguage(t *testing.T) {
	// Given a Portuguese assistant language
	// When building the prompt
	system := buildHintPrompt(testHintInput(), "pt")[0].Content

	// Then the hint is requested in Brazilian Portuguese
	assert.Contains(t, system, "Write the hint in Brazilian Portuguese.")
}

func TestBuildHintPrompt_userMessageHoldsChallengeRubricBoardAndEarlierHintsEscaped(t *testing.T) {
	// Given a hint input with earlier hints and a board that needs escaping
	// When building the prompt
	user := buildHintPrompt(testHintInput(), "en")[1].Content

	// Then every part is in its own tag, escaped
	assert.Contains(t, user, "<title>Design a URL shortener</title>")
	assert.Contains(t, user, "<scenario>\nA link service with 10M daily users.\n</scenario>")
	assert.Contains(t, user, "<requirement>Shorten a URL</requirement>")
	assert.Contains(t, user, "<requirement>p99 under 100 ms</requirement>")
	assert.Contains(t, user, "<constraint>No new database</constraint>")
	assert.Contains(t, user, "<objective>Cut read load</objective>")
	assert.Contains(t, user, "<criterion>Scalability: Handles the load</criterion>")
	assert.Contains(t, user, "<board>\n&lt;mxfile&gt;&lt;diagram&gt;a &amp; b&lt;/diagram&gt;&lt;/mxfile&gt;\n</board>")
	assert.Contains(t, user, "<hint>What happens if the cache is down?</hint>")
	assert.Contains(t, user, "<hint>Who owns the write path?</hint>")
	assert.Less(t, strings.Index(user, "cache is down"), strings.Index(user, "write path"))
}

func TestBuildHintPrompt_withoutBoardTellsTheTutorToPointAtHowToStart(t *testing.T) {
	// Given a request with no board content
	input := testHintInput()
	input.BoardXML = "  \n"

	// When building the prompt
	user := buildHintPrompt(input, "en")[1].Content

	// Then the board is reported empty instead of an empty tag
	assert.Contains(t, user, "<board>The board is empty. Point at how to start.</board>")
	assert.NotContains(t, user, "<board>\n")
}

func TestBuildHintPrompt_withoutEarlierHintsOmitsTheHintsTag(t *testing.T) {
	// Given a first hint request
	input := testHintInput()
	input.Previous = nil

	// When building the prompt
	user := buildHintPrompt(input, "en")[1].Content

	// Then there is no earlier-hints section
	assert.NotContains(t, user, "<hints_given>")
}

func TestHintTask_routesToTheArchitectureLabModel(t *testing.T) {
	// Given the hint call's task
	// When reading it
	// Then it uses the Architecture Lab model
	assert.Equal(t, domainllm.TaskArchitectureLab, hintTask)
}
