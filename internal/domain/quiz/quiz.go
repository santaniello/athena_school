package quiz

import (
	"errors"
	"strings"
	"time"
)

// Bounds on how many questions a quiz has.
const (
	MinQuestions     = 3
	MaxQuestions     = 20
	DefaultQuestions = 10
)

var (
	// ErrQuizSessionRequired is returned when a quiz has no source session.
	ErrQuizSessionRequired = errors.New("quiz session is required")
	// ErrQuizSessionTitleRequired is returned when a quiz has no session title snapshot.
	ErrQuizSessionTitleRequired = errors.New("quiz session title is required")
	// ErrDifficultyInvalid is returned for an unknown difficulty.
	ErrDifficultyInvalid = errors.New("quiz difficulty is invalid")
	// ErrQuestionCountInvalid is returned when the number of questions is out of range.
	ErrQuestionCountInvalid = errors.New("quiz question count is out of range")
	// ErrQuestionIDDuplicate is returned when two questions share an ID.
	ErrQuestionIDDuplicate = errors.New("quiz question id is duplicated")
	// ErrQuestionPositionInvalid is returned when positions are not 1..n in order.
	ErrQuestionPositionInvalid = errors.New("quiz question position is invalid")
)

// Difficulty is how demanding a quiz is.
type Difficulty string

// The difficulties of a quiz.
const (
	DifficultyEasy   Difficulty = "EASY"
	DifficultyMedium Difficulty = "MEDIUM"
	DifficultyHard   Difficulty = "HARD"
)

func (d Difficulty) valid() bool {
	return d == DifficultyEasy || d == DifficultyMedium || d == DifficultyHard
}

// Quiz is an exam generated from one study session. The session title is a
// snapshot, so the quiz outlives its session.
type Quiz struct {
	ID           string
	SessionID    string
	SessionTitle string
	Difficulty   Difficulty
	Questions    []Question
	CreatedAt    time.Time
	// MessageID is the study-chat message whose offer produced the quiz. It is
	// empty for a quiz generated from the menu, and it is not a reference the
	// database enforces: the quiz outlives the session and its messages.
	MessageID string
}

// Validate reports the first rule the quiz breaks, questions included.
func (q Quiz) Validate() error {
	if strings.TrimSpace(q.SessionID) == "" {
		return ErrQuizSessionRequired
	}
	if strings.TrimSpace(q.SessionTitle) == "" {
		return ErrQuizSessionTitleRequired
	}
	if !q.Difficulty.valid() {
		return ErrDifficultyInvalid
	}
	if len(q.Questions) < MinQuestions || len(q.Questions) > MaxQuestions {
		return ErrQuestionCountInvalid
	}
	ids := make(map[string]bool, len(q.Questions))
	for i, question := range q.Questions {
		if err := question.Validate(); err != nil {
			return questionError(i+1, err)
		}
		if ids[question.ID] {
			return questionError(i+1, ErrQuestionIDDuplicate)
		}
		ids[question.ID] = true
		if question.Position != i+1 {
			return questionError(i+1, ErrQuestionPositionInvalid)
		}
	}
	return nil
}

// Spec is what the user asks for when creating a quiz.
type Spec struct {
	Count      int
	Difficulty Difficulty
}

// Normalized returns the spec with a missing count read as DefaultQuestions
// and a missing difficulty as medium.
func (s Spec) Normalized() Spec {
	if s.Count == 0 {
		s.Count = DefaultQuestions
	}
	if s.Difficulty == "" {
		s.Difficulty = DifficultyMedium
	}
	return s
}

// Validate reports the first rule the normalized spec breaks.
func (s Spec) Validate() error {
	if s.Count < MinQuestions || s.Count > MaxQuestions {
		return ErrQuestionCountInvalid
	}
	if !s.Difficulty.valid() {
		return ErrDifficultyInvalid
	}
	return nil
}
