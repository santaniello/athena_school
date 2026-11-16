package quiz

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

// Service implements the quiz catalog, submission and history use cases.
type Service struct {
	repo  domainquiz.Repository
	now   func() time.Time
	newID func() string
}

// NewService creates a Service backed by repo.
func NewService(repo domainquiz.Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }, newID: uuid.NewString}
}

// AttemptResult is a graded attempt together with its quiz, which carries the
// answer key and the explanations.
type AttemptResult struct {
	Quiz    domainquiz.Quiz
	Attempt domainquiz.Attempt
}

// ListQuizzes returns every quiz, newest first.
func (s *Service) ListQuizzes(ctx context.Context) ([]domainquiz.Summary, error) {
	summaries, err := s.repo.ListQuizzes(ctx)
	if err != nil {
		return nil, fmt.Errorf("quiz: listing quizzes: %w", err)
	}
	return summaries, nil
}

// GetQuiz returns a quiz with its questions and answer key, or
// domainquiz.ErrQuizNotFound. Hiding the key while the user answers is the
// caller's job.
func (s *Service) GetQuiz(ctx context.Context, id string) (domainquiz.Quiz, error) {
	quiz, err := s.repo.GetQuiz(ctx, id)
	if err != nil {
		return domainquiz.Quiz{}, fmt.Errorf("quiz: reading quiz: %w", err)
	}
	return quiz, nil
}

// SubmitAttempt grades the selections against the quiz and stores the attempt.
// It returns domainquiz.ErrQuizNotFound for an unknown quiz and stores nothing
// when a selection is invalid.
func (s *Service) SubmitAttempt(ctx context.Context, quizID string, selections []domainquiz.Selection) (domainquiz.Attempt, error) {
	quiz, err := s.repo.GetQuiz(ctx, quizID)
	if err != nil {
		return domainquiz.Attempt{}, fmt.Errorf("quiz: reading quiz: %w", err)
	}
	attempt, err := domainquiz.Grade(quiz, selections)
	if err != nil {
		return domainquiz.Attempt{}, err
	}
	attempt.ID = s.newID()
	attempt.FinishedAt = s.now()
	if err := s.repo.CreateAttempt(ctx, attempt); err != nil {
		return domainquiz.Attempt{}, fmt.Errorf("quiz: saving attempt: %w", err)
	}
	return attempt, nil
}

// ListAttempts returns the attempts of a quiz, newest first.
func (s *Service) ListAttempts(ctx context.Context, quizID string) ([]domainquiz.Attempt, error) {
	attempts, err := s.repo.ListAttempts(ctx, quizID)
	if err != nil {
		return nil, fmt.Errorf("quiz: listing attempts: %w", err)
	}
	return attempts, nil
}

// GetAttemptResult returns an attempt with its quiz, so the answer key and the
// explanations can be shown. It returns domainquiz.ErrAttemptNotFound for an
// unknown attempt.
func (s *Service) GetAttemptResult(ctx context.Context, attemptID string) (AttemptResult, error) {
	attempt, err := s.repo.GetAttempt(ctx, attemptID)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("quiz: reading attempt: %w", err)
	}
	quiz, err := s.repo.GetQuiz(ctx, attempt.QuizID)
	if err != nil {
		return AttemptResult{}, fmt.Errorf("quiz: reading quiz of attempt: %w", err)
	}
	return AttemptResult{Quiz: quiz, Attempt: attempt}, nil
}

// DeleteQuiz removes a quiz and everything under it.
func (s *Service) DeleteQuiz(ctx context.Context, id string) error {
	if err := s.repo.DeleteQuiz(ctx, id); err != nil {
		return fmt.Errorf("quiz: deleting quiz: %w", err)
	}
	return nil
}
