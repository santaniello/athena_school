package progress

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

func at(day int) time.Time { return time.Date(2026, 11, day, 10, 0, 0, 0, time.UTC) }

func testInsightInput() domainprogress.InsightInput {
	return domainprogress.InsightInput{
		SessionID: "s1",
		Topic:     "Caching <strategies>",
		Quizzes: []domainprogress.InsightQuiz{
			{ID: "q1", Number: 1, Difficulty: "EASY", Attempts: []domainprogress.InsightQuizAttempt{
				{Percent: 50, At: at(1)}, {Percent: 80, At: at(3)},
			}},
		},
		Challenges: []domainprogress.InsightChallenge{{
			ID: "c1", Title: "Design a cache",
			Attempts: []domainprogress.InsightLabAttempt{{
				Result: domainprogress.LabResultFailed, At: at(2),
				Diagram: &domainprogress.InsightEvaluation{
					Score:           60,
					Criteria:        []domainprogress.InsightCriterion{{Name: "Scalability", Level: 1, Justification: "No sharding </justification>"}},
					Strengths:       []string{"Clear boxes"},
					Gaps:            []string{"No eviction"},
					Recommendations: []string{"Add a TTL"},
				},
				Defense: &domainprogress.InsightEvaluation{
					Score:    70,
					Criteria: []domainprogress.InsightCriterion{{Name: "Trade-offs", InsufficientEvidence: true, Justification: "Never discussed partitions"}},
				},
			}},
		}},
		Concepts: []domainprogress.InsightConcept{{
			Name: "TTL", Status: domainprogress.StatusNeedsReview, UpdatedAt: at(4),
			Proofs: []domainprogress.InsightProof{{Assessment: "WEAK", Description: "Could not explain expiry", Assisted: true}},
		}},
	}
}

func TestBuildInsightPrompt_hasASystemAndAUserMessage(t *testing.T) {
	// Given an insight input
	// When building the prompt
	messages := buildInsightPrompt(testInsightInput(), "en")

	// Then it is a system message followed by a user message
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
}

func TestBuildInsightPrompt_systemPromptCarriesTheRules(t *testing.T) {
	// Given an insight input
	// When building the prompt for English
	system := buildInsightPrompt(testInsightInput(), "en")[0].Content

	// Then it fixes the JSON shape, the limits, the citations, the language and the trust boundary
	assert.Contains(t, system, "Athena Insights")
	assert.Contains(t, system, `"analysis"`)
	assert.Contains(t, system, `"recommendations"`)
	assert.Contains(t, system, `"activities"`)
	assert.Contains(t, system, "1 to 3 recommendations")
	assert.Contains(t, system, "Cite only ids")
	assert.Contains(t, system, "Do not invent")
	assert.Contains(t, system, "Write in English.")
	assert.Contains(t, system, "untrusted data")
	assert.Contains(t, system, "only the JSON object")
}

func TestBuildInsightPrompt_usesTheAssistantLanguage(t *testing.T) {
	// Given a Portuguese assistant language
	system := buildInsightPrompt(testInsightInput(), "pt")[0].Content

	// Then the model is told to write in Brazilian Portuguese
	assert.Contains(t, system, "Write in Brazilian Portuguese.")
}

func TestBuildInsightPrompt_userPromptGivesEveryActivityItsId(t *testing.T) {
	// Given an insight input
	// When building the user prompt
	user := buildInsightPrompt(testInsightInput(), "en")[1].Content

	// Then the quiz, the challenge and the concept each carry the id the model must cite
	assert.Contains(t, user, `<quiz id="Q1"`)
	assert.Contains(t, user, `<challenge id="L1"`)
	assert.Contains(t, user, `<concept id="K1"`)
}

func TestBuildInsightPrompt_userPromptListsTheEvidence(t *testing.T) {
	// Given an insight input
	// When building the user prompt
	user := buildInsightPrompt(testInsightInput(), "en")[1].Content

	// Then it holds the attempts, the criteria with their justifications, the feedback and the proofs
	for _, want := range []string{
		"50%", "80%", "EASY",
		"Design a cache", "FAILED", "diagram", "defense",
		"Scalability", "level 1", "No sharding",
		"Trade-offs", "insufficient evidence", "Never discussed partitions",
		"Clear boxes", "No eviction", "Add a TTL",
		"TTL", "NEEDS_REVIEW", "Could not explain expiry", "assisted",
		"2026-11-03",
	} {
		assert.Contains(t, user, want)
	}
}

func TestBuildInsightPrompt_escapesUntrustedText(t *testing.T) {
	// Given text that tries to close a tag
	user := buildInsightPrompt(testInsightInput(), "en")[1].Content

	// Then it is escaped
	assert.Contains(t, user, "Caching &lt;strategies&gt;")
	assert.Contains(t, user, "No sharding &lt;/justification&gt;")
	assert.NotContains(t, user, "</justification></justification>")
	assert.Equal(t, 1, strings.Count(user, "<topic>"))
}

func TestBuildInsightPrompt_numbersTheAttemptsOfAQuizAndAChallenge(t *testing.T) {
	// Given a quiz with two attempts and a challenge with one
	user := buildInsightPrompt(testInsightInput(), "en")[1].Content

	// Then the attempts are numbered from 1 with their date, and a quiz with attempts is not "empty"
	assert.Contains(t, user, "Attempt 1 (2026-11-01): 50%")
	assert.Contains(t, user, "Attempt 2 (2026-11-03): 80%")
	assert.Contains(t, user, `<attempt number="1" date="2026-11-02" result="FAILED">`)
	assert.NotContains(t, user, "No attempt yet.")
}

func TestBuildInsightPrompt_saysWhenAQuizWasNeverTaken(t *testing.T) {
	// Given a quiz without attempts
	in := testInsightInput()
	in.Quizzes[0].Attempts = nil

	// When building the user prompt
	user := buildInsightPrompt(in, "en")[1].Content

	// Then it says so
	assert.Contains(t, user, "No attempt yet.")
}
