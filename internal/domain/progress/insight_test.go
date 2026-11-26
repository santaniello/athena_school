package progress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insightInput() InsightInput {
	return InsightInput{
		SessionID: "s1",
		Topic:     "Caching",
		Quizzes: []InsightQuiz{
			{ID: "q1", Number: 1, Difficulty: "EASY", Attempts: []InsightQuizAttempt{{Percent: 50, At: day(1)}, {Percent: 80, At: day(3)}}},
		},
		Challenges: []InsightChallenge{
			{ID: "c1", Title: "Design a cache", Attempts: []InsightLabAttempt{{Result: LabResultPassed, At: day(2)}}},
		},
		Concepts: []InsightConcept{
			{Name: "TTL", Status: StatusDemonstrated, UpdatedAt: day(4)},
		},
	}
}

func TestInsightRefs_numberEachKindOnItsOwnFromOne(t *testing.T) {
	// Given the activities of a topic
	in := insightInput()

	// When the refs of the first of each kind are read
	// Then each kind has its own letter and starts at 1
	assert.Equal(t, "Q1", in.QuizRef(0))
	assert.Equal(t, "L1", in.ChallengeRef(0))
	assert.Equal(t, "K1", in.ConceptRef(0))
	assert.Equal(t, "Q2", in.QuizRef(1))
}

func TestFingerprint_isStableForTheSameInputAndLanguage(t *testing.T) {
	// Given the same input twice
	// When the fingerprints are computed
	// Then they are equal and non-empty
	first := Fingerprint(insightInput(), "en")
	assert.NotEmpty(t, first)
	assert.Equal(t, first, Fingerprint(insightInput(), "en"))
}

func TestFingerprint_changesWithTheLanguage(t *testing.T) {
	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(insightInput(), "pt"))
}

func TestFingerprint_changesWhenAQuizIsRetaken(t *testing.T) {
	// Given a quiz retaken after the insight
	retaken := insightInput()
	retaken.Quizzes[0].Attempts = append(retaken.Quizzes[0].Attempts, InsightQuizAttempt{Percent: 90, At: day(9)})

	// Then the fingerprint differs
	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(retaken, "en"))
}

func TestFingerprint_changesWhenAChallengeIsAttemptedOrEvaluatedAgain(t *testing.T) {
	// Given a challenge whose latest attempt moved on
	moved := insightInput()
	moved.Challenges[0].Attempts[0].At = day(8)

	// Then the fingerprint differs
	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(moved, "en"))
}

func TestFingerprint_changesWhenAConceptIsUpdated(t *testing.T) {
	updated := insightInput()
	updated.Concepts[0].UpdatedAt = day(10)

	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(updated, "en"))
}

func TestFingerprint_changesWhenAProofIsRecordedWithoutAStatusChange(t *testing.T) {
	// Given a new proof of a concept whose own update time did not move
	proved := insightInput()
	proved.Concepts[0].Proofs = []InsightProof{{At: day(11)}}

	// Then the fingerprint differs
	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(proved, "en"))
}

func TestFingerprint_changesWhenTheResultOfALabAttemptChanges(t *testing.T) {
	// Given the same attempt that now passes instead of failing (a new pass score)
	failed := insightInput()
	failed.Challenges[0].Attempts[0].Result = LabResultFailed

	// Then the fingerprint differs
	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(failed, "en"))
}

func TestFingerprint_changesWhenAnActivityIsAdded(t *testing.T) {
	added := insightInput()
	added.Quizzes = append(added.Quizzes, InsightQuiz{ID: "q2", Number: 2})

	assert.NotEqual(t, Fingerprint(insightInput(), "en"), Fingerprint(added, "en"))
}

func TestFingerprint_ignoresTheTextOfTheEvidence(t *testing.T) {
	// Given the same activities with different prose
	reworded := insightInput()
	reworded.Topic = "Another name"
	reworded.Concepts[0].Proofs = []InsightProof{{Description: "something else"}}

	// Then the fingerprint is the same: only ids and timestamps count
	assert.Equal(t, Fingerprint(insightInput(), "en"), Fingerprint(reworded, "en"))
}

func TestFingerprint_doesNotConfuseKindsSharingAnId(t *testing.T) {
	// Given a quiz and a challenge with the same id, swapped
	a := InsightInput{Quizzes: []InsightQuiz{{ID: "x", Attempts: []InsightQuizAttempt{{At: day(1)}}}}}
	b := InsightInput{Challenges: []InsightChallenge{{ID: "x", Attempts: []InsightLabAttempt{{At: day(1)}}}}}

	assert.NotEqual(t, Fingerprint(a, "en"), Fingerprint(b, "en"))
}

const validInsightJSON = `{
  "analysis": [
    {"text": "Quiz results improved.", "activities": ["Q1"]},
    {"text": "The lab was passed.", "activities": ["L1", "K1"]}
  ],
  "recommendations": [
    {"text": "Review eviction policies.", "activities": ["K1"]}
  ]
}`

func TestParseInsight_buildsStatementsWithTheirCitations(t *testing.T) {
	// Given a valid answer
	// When it is parsed against the input
	content, err := ParseInsight(validInsightJSON, insightInput())

	// Then each statement keeps its text and cites the activities it names, with labels
	require.NoError(t, err)
	assert.Equal(t, []Statement{
		{Text: "Quiz results improved.", Citations: []Citation{{Kind: CitationQuiz, ID: "q1", Label: "Quiz 1"}}},
		{Text: "The lab was passed.", Citations: []Citation{
			{Kind: CitationChallenge, ID: "c1", Label: "Design a cache"},
			{Kind: CitationConcept, ID: "TTL", Label: "TTL"},
		}},
	}, content.Analysis)
	assert.Equal(t, []Statement{
		{Text: "Review eviction policies.", Citations: []Citation{{Kind: CitationConcept, ID: "TTL", Label: "TTL"}}},
	}, content.Recommendations)
}

func TestParseInsight_dropsCitationsOfUnknownActivities(t *testing.T) {
	// Given an answer citing an activity that does not exist, and one repeated
	raw := `{"analysis":[{"text":"A.","activities":["Q9","Q1","Q1","zzz"]}],"recommendations":[{"text":"B.","activities":[]}]}`

	// When it is parsed
	content, err := ParseInsight(raw, insightInput())

	// Then only the known activity stays, once, and the statement is kept
	require.NoError(t, err)
	assert.Equal(t, []Citation{{Kind: CitationQuiz, ID: "q1", Label: "Quiz 1"}}, content.Analysis[0].Citations)
	assert.NotNil(t, content.Recommendations[0].Citations)
	assert.Empty(t, content.Recommendations[0].Citations)
}

func TestParseInsight_trimsTheText(t *testing.T) {
	content, err := ParseInsight(`{"analysis":[{"text":"  A.  "}],"recommendations":[{"text":" B. "}]}`, insightInput())

	require.NoError(t, err)
	assert.Equal(t, "A.", content.Analysis[0].Text)
	assert.Equal(t, "B.", content.Recommendations[0].Text)
}

func TestParseInsight_rejectsInvalidAnswers(t *testing.T) {
	tests := map[string]string{
		"not json":              `Sure! Here is your analysis.`,
		"empty object":          `{}`,
		"no analysis":           `{"analysis":[],"recommendations":[{"text":"B."}]}`,
		"blank analysis":        `{"analysis":[{"text":"  "}],"recommendations":[{"text":"B."}]}`,
		"no recommendations":    `{"analysis":[{"text":"A."}],"recommendations":[]}`,
		"blank recommendation":  `{"analysis":[{"text":"A."}],"recommendations":[{"text":""}]}`,
		"four recommendations":  `{"analysis":[{"text":"A."}],"recommendations":[{"text":"1"},{"text":"2"},{"text":"3"},{"text":"4"}]}`,
		"trailing text":         validInsightJSON + ` and more`,
		"analysis is a string":  `{"analysis":"A.","recommendations":[{"text":"B."}]}`,
		"recommendation string": `{"analysis":[{"text":"A."}],"recommendations":["B."]}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			// When an invalid answer is parsed
			_, err := ParseInsight(raw, insightInput())

			// Then it is ErrInvalidInsight
			require.ErrorIs(t, err, ErrInvalidInsight)
		})
	}
}

func TestParseInsight_boundsTheAnalysis(t *testing.T) {
	sentences := func(n int) string {
		return strings.TrimSuffix(strings.Repeat(`{"text":"A."},`, n), ",")
	}
	raw := func(n int) string {
		return `{"analysis":[` + sentences(n) + `],"recommendations":[{"text":"B."}]}`
	}

	_, err := ParseInsight(raw(MaxInsightAnalysis), insightInput())
	require.NoError(t, err)

	_, err = ParseInsight(raw(MaxInsightAnalysis+1), insightInput())
	require.ErrorIs(t, err, ErrInvalidInsight)
}

func TestParseInsight_acceptsExactlyThreeRecommendations(t *testing.T) {
	_, err := ParseInsight(`{"analysis":[{"text":"A."}],"recommendations":[{"text":"1"},{"text":"2"},{"text":"3"}]}`, insightInput())

	require.NoError(t, err)
}

func TestParseInsight_namesTheBlankItemFromOne(t *testing.T) {
	// Given a recommendation list whose second item is blank
	_, err := ParseInsight(`{"analysis":[{"text":"A."}],"recommendations":[{"text":"1"},{"text":" "}]}`, insightInput())

	// Then the error points at it by its position from 1
	require.ErrorIs(t, err, ErrInvalidInsight)
	assert.ErrorContains(t, err, "item 2 is blank")
}
