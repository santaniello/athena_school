package quiz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validChoice() Question {
	return Question{
		ID: "q1", Position: 1, Kind: KindMultipleChoice,
		Prompt: "Which cache policy evicts the oldest entry?", Options: []string{"LRU", "FIFO", "LFU"},
		CorrectIndex: 1, Explanation: "FIFO evicts by insertion order.",
	}
}

func validTrueFalse() Question {
	return Question{
		ID: "q2", Position: 2, Kind: KindTrueFalse,
		Prompt: "A cache always improves latency.", Options: []string{"True", "False"},
		CorrectIndex: 1, Explanation: "A cold cache adds a hop.",
	}
}

func TestQuestion_Validate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *Question)
		base   func() Question
		want   error
	}{
		{"valid multiple choice", func(*Question) {}, validChoice, nil},
		{"valid true or false", func(*Question) {}, validTrueFalse, nil},
		{"blank prompt", func(q *Question) { q.Prompt = "  " }, validChoice, ErrQuestionPromptRequired},
		{"prompt too long", func(q *Question) { q.Prompt = strings.Repeat("a", MaxPromptLength+1) }, validChoice, ErrQuestionPromptTooLong},
		{"prompt at the limit", func(q *Question) { q.Prompt = strings.Repeat("a", MaxPromptLength) }, validChoice, nil},
		{"blank explanation", func(q *Question) { q.Explanation = "" }, validChoice, ErrQuestionExplanationRequired},
		{"unknown kind", func(q *Question) { q.Kind = "ESSAY" }, validChoice, ErrQuestionKindInvalid},
		{"one option", func(q *Question) { q.Options = []string{"A"}; q.CorrectIndex = 0 }, validChoice, ErrQuestionOptionsInvalid},
		{"two options", func(q *Question) { q.Options = []string{"A", "B"} }, validChoice, nil},
		{"six options", func(q *Question) { q.Options = []string{"A", "B", "C", "D", "E", "F"} }, validChoice, nil},
		{"seven options", func(q *Question) { q.Options = []string{"A", "B", "C", "D", "E", "F", "G"} }, validChoice, ErrQuestionOptionsInvalid},
		{"true or false with three options", func(q *Question) { q.Options = []string{"T", "F", "M"} }, validTrueFalse, ErrQuestionOptionsInvalid},
		{"blank option", func(q *Question) { q.Options[0] = " " }, validChoice, ErrQuestionOptionEmpty},
		{"duplicate option", func(q *Question) { q.Options[2] = " lru"; q.Options[0] = "LRU" }, validChoice, ErrQuestionOptionDuplicate},
		{"option too long", func(q *Question) { q.Options[0] = strings.Repeat("a", MaxOptionLength+1) }, validChoice, ErrQuestionOptionTooLong},
		{"option at the limit", func(q *Question) { q.Options[0] = strings.Repeat("a", MaxOptionLength) }, validChoice, nil},
		{"correct index negative", func(q *Question) { q.CorrectIndex = -1 }, validChoice, ErrQuestionCorrectIndexInvalid},
		{"correct index is the first option", func(q *Question) { q.CorrectIndex = 0 }, validChoice, nil},
		{"correct index is the last option", func(q *Question) { q.CorrectIndex = 2 }, validChoice, nil},
		{"correct index past the options", func(q *Question) { q.CorrectIndex = 3 }, validChoice, ErrQuestionCorrectIndexInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a question with one field changed
			q := tt.base()
			tt.mutate(&q)

			// When it is validated
			err := q.Validate()

			// Then it reports the first broken rule, or none
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestQuestion_Shuffled_movesTheCorrectOptionWithItsText(t *testing.T) {
	// Given a multiple-choice question whose correct option is "FIFO" (index 1)
	q := validChoice()

	// When the options are reordered by the permutation [2, 0, 1]
	shuffled, err := q.Shuffled([]int{2, 0, 1})

	// Then the options follow the permutation and the correct index follows FIFO
	assert.NoError(t, err)
	assert.Equal(t, []string{"LFU", "LRU", "FIFO"}, shuffled.Options)
	assert.Equal(t, 2, shuffled.CorrectIndex)
	assert.Equal(t, []string{"LRU", "FIFO", "LFU"}, q.Options, "the original is not changed")
}

func TestQuestion_Shuffled_keepsTrueFalseOrder(t *testing.T) {
	// Given a true/false question
	q := validTrueFalse()

	// When it is shuffled with a swapping permutation
	shuffled, err := q.Shuffled([]int{1, 0})

	// Then True stays before False
	assert.NoError(t, err)
	assert.Equal(t, q, shuffled)
}

func TestQuestion_Shuffled_rejectsAnInvalidPermutation(t *testing.T) {
	for name, perm := range map[string][]int{
		"too short":    {0, 1},
		"repeats":      {0, 0, 1},
		"out of range": {0, 1, 3},
		"negative":     {0, 1, -1},
	} {
		t.Run(name, func(t *testing.T) {
			// When a multiple-choice question is shuffled with a bad permutation
			_, err := validChoice().Shuffled(perm)

			// Then it is refused
			assert.ErrorIs(t, err, ErrPermutationInvalid)
		})
	}
}
