package quiz

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrQuizNotFound is returned when no quiz matches the given ID.
	ErrQuizNotFound = errors.New("quiz not found")
	// ErrAttemptNotFound is returned when no attempt matches the given ID.
	ErrAttemptNotFound = errors.New("quiz attempt not found")
)

// Summary is a quiz as the catalog lists it, without its questions.
type Summary struct {
	ID            string
	SessionID     string
	SessionTitle  string
	Difficulty    Difficulty
	QuestionCount int
	CreatedAt     time.Time
	AttemptCount  int
	LatestScore   *float64 // nil until the quiz has an attempt
	MessageID     string   // empty unless an offer in the study chat produced the quiz
}

// Repository stores quizzes and their attempts. A quiz is saved with its
// questions in one step, and deleting a quiz deletes its questions, attempts
// and answers.
type Repository interface {
	// CreateQuiz stores the quiz and its questions.
	CreateQuiz(ctx context.Context, quiz Quiz) error
	// GetQuiz returns the quiz with its questions, or ErrQuizNotFound.
	GetQuiz(ctx context.Context, id string) (Quiz, error)
	// ListQuizzes returns every quiz, newest first.
	ListQuizzes(ctx context.Context) ([]Summary, error)
	// FindByMessage returns the quiz an offer produced under the chat message,
	// or ErrQuizNotFound.
	FindByMessage(ctx context.Context, messageID string) (Summary, error)
	// ListBySession returns the quizzes an offer produced in the session, in the
	// order of their messages' creation (oldest first).
	ListBySession(ctx context.Context, sessionID string) ([]Summary, error)
	// DeleteQuiz removes the quiz and everything under it, or returns ErrQuizNotFound.
	DeleteQuiz(ctx context.Context, id string) error
	// CreateAttempt stores a graded attempt and its answers.
	CreateAttempt(ctx context.Context, attempt Attempt) error
	// GetAttempt returns the attempt with its answers, or ErrAttemptNotFound.
	GetAttempt(ctx context.Context, id string) (Attempt, error)
	// ListAttempts returns the attempts of a quiz, newest first, without their
	// answers: it is a summary list. GetAttempt returns the answers.
	ListAttempts(ctx context.Context, quizID string) ([]Attempt, error)
}
