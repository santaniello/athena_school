package quiz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func pick(i int) *int { return &i }

func TestScore(t *testing.T) {
	tests := []struct {
		correct, total int
		want           float64
	}{
		{0, 10, 0},
		{10, 10, 10},
		{1, 3, 3.3},
		{2, 3, 6.7},
		{1, 6, 1.7},
		{7, 8, 8.8},
		{1, 8, 1.3},
		{5, 10, 5},
	}
	for _, tt := range tests {
		// When the score of correct answers out of total is computed
		got := Score(tt.correct, tt.total)

		// Then it is on a 0-10 scale with one decimal, rounded half up
		assert.Equal(t, tt.want, got, "%d/%d", tt.correct, tt.total)
	}
}

func TestScore_noQuestionsIsZero(t *testing.T) {
	assert.Equal(t, 0.0, Score(0, 0))
}

func TestGrade_countsCorrectWrongAndBlank(t *testing.T) {
	// Given a 4-question quiz whose correct index is 1 on every question
	quiz := validQuiz(4)

	// When the user answers right, wrong, blank (omitted) and explicitly blank
	attempt, err := Grade(quiz, []Selection{
		{QuestionID: "q1", SelectedIndex: pick(1)},
		{QuestionID: "q2", SelectedIndex: pick(0)},
		{QuestionID: "q4", SelectedIndex: nil},
	})

	// Then only the right answer counts and the score is 1 of 4 on the 0-10 scale
	assert.NoError(t, err)
	assert.Equal(t, 1, attempt.CorrectCount)
	assert.Equal(t, 4, attempt.Total)
	assert.Equal(t, 2.5, attempt.Score)
	assert.Equal(t, []Answer{
		{QuestionID: "q1", SelectedIndex: pick(1), Correct: true},
		{QuestionID: "q2", SelectedIndex: pick(0), Correct: false},
		{QuestionID: "q3", SelectedIndex: nil, Correct: false},
		{QuestionID: "q4", SelectedIndex: nil, Correct: false},
	}, attempt.Answers)
	assert.Equal(t, "z1", attempt.QuizID)
}

func TestGrade_allCorrectIsTen(t *testing.T) {
	quiz := validQuiz(3)
	attempt, err := Grade(quiz, []Selection{
		{QuestionID: "q1", SelectedIndex: pick(1)},
		{QuestionID: "q2", SelectedIndex: pick(1)},
		{QuestionID: "q3", SelectedIndex: pick(1)},
	})
	assert.NoError(t, err)
	assert.Equal(t, 10.0, attempt.Score)
	assert.Equal(t, 3, attempt.CorrectCount)
}

func TestGrade_answersFollowTheQuestionOrderNotTheSelectionOrder(t *testing.T) {
	quiz := validQuiz(3)
	attempt, err := Grade(quiz, []Selection{
		{QuestionID: "q3", SelectedIndex: pick(1)},
		{QuestionID: "q1", SelectedIndex: pick(0)},
	})
	assert.NoError(t, err)
	assert.Equal(t, "q1", attempt.Answers[0].QuestionID)
	assert.Equal(t, "q2", attempt.Answers[1].QuestionID)
	assert.Equal(t, "q3", attempt.Answers[2].QuestionID)
}

func TestGrade_refusesABadSelection(t *testing.T) {
	tests := []struct {
		name       string
		selections []Selection
		want       error
	}{
		{"unknown question", []Selection{{QuestionID: "nope", SelectedIndex: pick(0)}}, ErrSelectionUnknownQuestion},
		{"answered twice", []Selection{{QuestionID: "q1", SelectedIndex: pick(0)}, {QuestionID: "q1", SelectedIndex: pick(1)}}, ErrSelectionDuplicate},
		{"negative option", []Selection{{QuestionID: "q1", SelectedIndex: pick(-1)}}, ErrSelectionOutOfRange},
		{"option past the last", []Selection{{QuestionID: "q1", SelectedIndex: pick(3)}}, ErrSelectionOutOfRange},
		{"the last option is fine", []Selection{{QuestionID: "q1", SelectedIndex: pick(2)}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When a 3-option quiz is graded with that selection
			_, err := Grade(validQuiz(3), tt.selections)

			// Then a bad selection is refused
			assert.ErrorIs(t, err, tt.want)
		})
	}
}
