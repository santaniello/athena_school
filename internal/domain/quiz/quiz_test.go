package quiz

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func questions(n int) []Question {
	out := make([]Question, n)
	for i := range out {
		q := validChoice()
		q.ID = fmt.Sprintf("q%d", i+1)
		q.Position = i + 1
		out[i] = q
	}
	return out
}

func validQuiz(n int) Quiz {
	return Quiz{
		ID: "z1", SessionID: "s1", SessionTitle: "Caching", Difficulty: DifficultyMedium,
		Questions: questions(n), CreatedAt: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
	}
}

func TestQuiz_Validate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *Quiz)
		want   error
	}{
		{"valid", func(*Quiz) {}, nil},
		{"no session", func(q *Quiz) { q.SessionID = " " }, ErrQuizSessionRequired},
		{"no session title", func(q *Quiz) { q.SessionTitle = "" }, ErrQuizSessionTitleRequired},
		{"unknown difficulty", func(q *Quiz) { q.Difficulty = "EXTREME" }, ErrDifficultyInvalid},
		{"too few questions", func(q *Quiz) { q.Questions = questions(MinQuestions - 1) }, ErrQuestionCountInvalid},
		{"minimum questions", func(q *Quiz) { q.Questions = questions(MinQuestions) }, nil},
		{"maximum questions", func(q *Quiz) { q.Questions = questions(MaxQuestions) }, nil},
		{"too many questions", func(q *Quiz) { q.Questions = questions(MaxQuestions + 1) }, ErrQuestionCountInvalid},
		{"invalid question", func(q *Quiz) { q.Questions[1].Prompt = "" }, ErrQuestionPromptRequired},
		{"duplicate question id", func(q *Quiz) { q.Questions[1].ID = "q1" }, ErrQuestionIDDuplicate},
		{"positions out of order", func(q *Quiz) { q.Questions[1].Position = 5 }, ErrQuestionPositionInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a quiz with one field changed
			q := validQuiz(5)
			tt.mutate(&q)

			// When it is validated
			err := q.Validate()

			// Then it reports the first broken rule, or none
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestSpec_Normalized_defaultsTheCount(t *testing.T) {
	// Given a spec that asks for no count
	spec := Spec{Difficulty: DifficultyHard}

	// When it is normalized
	got := spec.Normalized()

	// Then the count is the default
	assert.Equal(t, Spec{Count: DefaultQuestions, Difficulty: DifficultyHard}, got)
}

func TestSpec_Normalized_keepsAGivenCount(t *testing.T) {
	assert.Equal(t, 7, Spec{Count: 7, Difficulty: DifficultyEasy}.Normalized().Count)
}

func TestSpec_Normalized_defaultsTheDifficulty(t *testing.T) {
	assert.Equal(t, DifficultyMedium, Spec{Count: 5}.Normalized().Difficulty)
}

func TestSpec_Validate(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		want error
	}{
		{"valid", Spec{Count: 10, Difficulty: DifficultyEasy}, nil},
		{"minimum", Spec{Count: MinQuestions, Difficulty: DifficultyEasy}, nil},
		{"maximum", Spec{Count: MaxQuestions, Difficulty: DifficultyHard}, nil},
		{"below minimum", Spec{Count: MinQuestions - 1, Difficulty: DifficultyEasy}, ErrQuestionCountInvalid},
		{"above maximum", Spec{Count: MaxQuestions + 1, Difficulty: DifficultyEasy}, ErrQuestionCountInvalid},
		{"unknown difficulty", Spec{Count: 5, Difficulty: "X"}, ErrDifficultyInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, tt.spec.Validate(), tt.want)
		})
	}
}

func TestQuiz_Validate_namesThePositionOfTheBrokenQuestion(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *Quiz)
		want   string
	}{
		{"invalid question", func(q *Quiz) { q.Questions[1].Prompt = "" }, "question 2: "},
		{"duplicate id", func(q *Quiz) { q.Questions[2].ID = "q1" }, "question 3: "},
		{"position", func(q *Quiz) { q.Questions[3].Position = 9 }, "question 4: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a quiz whose Nth question breaks a rule
			q := validQuiz(5)
			tt.mutate(&q)

			// When it is validated
			err := q.Validate()

			// Then the message says which question, counting from 1
			assert.ErrorContains(t, err, tt.want)
		})
	}
}
