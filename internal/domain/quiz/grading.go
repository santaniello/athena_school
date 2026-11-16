package quiz

import (
	"errors"
	"math"
	"time"
)

var (
	// ErrSelectionUnknownQuestion is returned when a selection names a question the quiz lacks.
	ErrSelectionUnknownQuestion = errors.New("selection names an unknown question")
	// ErrSelectionDuplicate is returned when a question is answered twice.
	ErrSelectionDuplicate = errors.New("question is answered more than once")
	// ErrSelectionOutOfRange is returned when a chosen option is not one of the question's options.
	ErrSelectionOutOfRange = errors.New("selected option is out of range")
)

// maxScore is the top of the grade scale.
const maxScore = 10

// Selection is what the user chose for one question. A nil SelectedIndex is a
// blank answer.
type Selection struct {
	QuestionID    string
	SelectedIndex *int
}

// Answer is a graded selection.
type Answer struct {
	QuestionID    string
	SelectedIndex *int // nil when left blank
	Correct       bool
}

// Attempt is one graded submission of a quiz.
type Attempt struct {
	ID           string
	QuizID       string
	Answers      []Answer // in the quiz's question order
	CorrectCount int
	Total        int
	Score        float64 // 0 to 10, one decimal
	FinishedAt   time.Time
}

// Score is correct out of total on a 0-10 scale, rounded to one decimal, half
// up. It is 0 when there are no questions.
func Score(correct, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(correct)*maxScore*10/float64(total)) / 10
}

// Grade grades the selections against the quiz. A question with no selection,
// or a nil one, is blank and counts as wrong. It refuses a selection that
// names an unknown question, answers one twice or picks an option the question
// does not have. The attempt's ID and FinishedAt are left for the caller.
func Grade(quiz Quiz, selections []Selection) (Attempt, error) {
	byQuestion, err := indexSelections(quiz, selections)
	if err != nil {
		return Attempt{}, err
	}
	attempt := Attempt{QuizID: quiz.ID, Total: len(quiz.Questions), Answers: make([]Answer, 0, len(quiz.Questions))}
	for _, question := range quiz.Questions {
		selected := byQuestion[question.ID]
		correct := selected != nil && *selected == question.CorrectIndex
		if correct {
			attempt.CorrectCount++
		}
		attempt.Answers = append(attempt.Answers, Answer{QuestionID: question.ID, SelectedIndex: selected, Correct: correct})
	}
	attempt.Score = Score(attempt.CorrectCount, attempt.Total)
	return attempt, nil
}

func indexSelections(quiz Quiz, selections []Selection) (map[string]*int, error) {
	questions := make(map[string]Question, len(quiz.Questions))
	for _, question := range quiz.Questions {
		questions[question.ID] = question
	}
	byQuestion := make(map[string]*int, len(selections))
	for _, selection := range selections {
		question, ok := questions[selection.QuestionID]
		if !ok {
			return nil, ErrSelectionUnknownQuestion
		}
		if _, answered := byQuestion[selection.QuestionID]; answered {
			return nil, ErrSelectionDuplicate
		}
		if selection.SelectedIndex != nil && (*selection.SelectedIndex < 0 || *selection.SelectedIndex >= len(question.Options)) {
			return nil, ErrSelectionOutOfRange
		}
		byQuestion[selection.QuestionID] = selection.SelectedIndex
	}
	return byQuestion, nil
}
