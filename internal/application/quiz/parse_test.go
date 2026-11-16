package quiz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

const threeQuestions = `{"questions":[
 {"kind":"MULTIPLE_CHOICE","prompt":" Which policy evicts the oldest? ","options":[" LRU","FIFO "],"correctIndex":1,"explanation":" FIFO uses insertion order. "},
 {"kind":"TRUE_FALSE","prompt":"A cache is always faster.","options":["True","False"],"correctIndex":1,"explanation":"Cold caches add a hop."},
 {"kind":"MULTIPLE_CHOICE","prompt":"What is a cache hit?","options":["A","B","C"],"correctIndex":0,"explanation":"The key was found."}
]}`

func TestParseQuestions_readsTheQuestionsAndTrimsText(t *testing.T) {
	// When a valid answer is parsed
	questions, err := parseQuestions(threeQuestions, 3)

	// Then the questions come back in order with trimmed text
	require.NoError(t, err)
	require.Len(t, questions, 3)
	assert.Equal(t, domainquiz.Question{
		Kind: domainquiz.KindMultipleChoice, Prompt: "Which policy evicts the oldest?",
		Options: []string{"LRU", "FIFO"}, CorrectIndex: 1, Explanation: "FIFO uses insertion order.",
	}, questions[0])
	assert.Equal(t, domainquiz.KindTrueFalse, questions[1].Kind)
	assert.Equal(t, 0, questions[2].CorrectIndex)
}

func TestParseQuestions_acceptsACodeFence(t *testing.T) {
	for _, wrapped := range []string{"```json\n" + threeQuestions + "\n```", "```\n" + threeQuestions + "```"} {
		questions, err := parseQuestions(wrapped, 3)
		require.NoError(t, err)
		assert.Len(t, questions, 3)
	}
}

func TestParseQuestions_refusesABadAnswer(t *testing.T) {
	tests := map[string]struct {
		content string
		count   int
	}{
		"not json":          {"here are your questions", 3},
		"no questions key":  {`{"items":[]}`, 3},
		"too few questions": {threeQuestions, 4},
		"too many":          {threeQuestions, 2},
		"missing correct index": {
			`{"questions":[{"kind":"TRUE_FALSE","prompt":"p","options":["True","False"],"explanation":"e"}]}`, 1,
		},
		"null correct index": {
			`{"questions":[{"kind":"TRUE_FALSE","prompt":"p","options":["True","False"],"correctIndex":null,"explanation":"e"}]}`, 1,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// When the answer is parsed
			_, err := parseQuestions(tt.content, tt.count)

			// Then it is invalid generation and nothing is returned
			assert.ErrorIs(t, err, ErrInvalidGeneration)
		})
	}
}

func TestStripCodeFence(t *testing.T) {
	assert.Equal(t, `{"a":1}`, string(stripCodeFence(`  {"a":1}  `)))
	assert.Equal(t, `{"a":1}`, string(stripCodeFence("```json\n{\"a\":1}\n```")))
	assert.Equal(t, `{"a":1}`, string(stripCodeFence("```{\"a\":1}```")))
	assert.False(t, strings.HasPrefix(string(stripCodeFence("```\nx\n```")), "`"))
}

func TestParseQuestions_namesTheQuestionWithoutACorrectIndex(t *testing.T) {
	// Given an answer whose second question has no correct index
	content := `{"questions":[
 {"kind":"TRUE_FALSE","prompt":"p","options":["True","False"],"correctIndex":0,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"q","options":["True","False"],"explanation":"e"}]}`

	// When it is parsed
	_, err := parseQuestions(content, 2)

	// Then the message counts the questions from 1
	require.ErrorIs(t, err, ErrInvalidGeneration)
	assert.ErrorContains(t, err, "question 2 has no correct index")
}
