package exercise

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// AttemptService runs one exam attempt: start, submit, discard and clean up.
// It enforces the exam rules of ADR-005 (decision 9). Scoring starts in
// 3.5, so a submitted attempt stays SUBMITTED here. See
// specs/phases/phase-03-architecture-lab/04-exercise-service.md.
type AttemptService struct {
	challenges  domainexercise.ChallengeRepository
	rubrics     domainexercise.RubricRepository
	attempts    domainexercise.AttemptRepository
	submissions domainexercise.SubmissionRepository
	links       domainexercise.SessionExerciseRepository
	sessions    SessionExists
	tx          Transactor
	clock       Clock
	newID       func() string
}

// NewAttemptService creates an AttemptService backed by the given ports.
func NewAttemptService(
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	submissions domainexercise.SubmissionRepository,
	links domainexercise.SessionExerciseRepository,
	sessions SessionExists,
	tx Transactor,
	clock Clock,
) *AttemptService {
	return &AttemptService{
		challenges:  challenges,
		rubrics:     rubrics,
		attempts:    attempts,
		submissions: submissions,
		links:       links,
		sessions:    sessions,
		tx:          tx,
		clock:       clock,
		newID:       uuid.NewString,
	}
}

// StartInput says which challenge to start and from where.
type StartInput struct {
	ChallengeID       string
	SessionID         string // "" for a standalone attempt
	DiscardInProgress bool
}

// StartAttempt starts an exam on the challenge, frozen to its latest rubric.
func (s *AttemptService) StartAttempt(ctx context.Context, input StartInput) (AttemptView, error) {
	challenge, err := s.challenges.Get(ctx, input.ChallengeID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Latest(ctx, input.ChallengeID)
	if errors.Is(err, domainexercise.ErrRubricNotFound) {
		return AttemptView{}, ErrChallengeHasNoRubric
	}
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting latest rubric: %w", err)
	}

	running, err := s.attempts.FindInProgress(ctx)
	hasRunning := err == nil
	if err != nil && !errors.Is(err, domainexercise.ErrAttemptNotFound) {
		return AttemptView{}, fmt.Errorf("exercise: looking for a running attempt: %w", err)
	}
	if hasRunning && !input.DiscardInProgress {
		return AttemptView{}, &AttemptInProgressError{AttemptID: running.ID}
	}

	attempt, err := domainexercise.NewAttempt(challenge, rubric, "", s.clock.Now())
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: creating attempt: %w", err)
	}
	attempt.ID = s.newID()
	if err := s.replaceAndStore(ctx, running, hasRunning, attempt); err != nil {
		return AttemptView{}, err
	}
	return viewOf(attempt, challenge, rubric, nil), nil
}

// replaceAndStore deletes the running attempt, if any, and stores the new one
// in a single transaction, so a failed create leaves the old attempt in place.
// Losing a race against a concurrent start surfaces as ErrAttemptInProgress.
func (s *AttemptService) replaceAndStore(ctx context.Context, running domainexercise.ChallengeAttempt, hasRunning bool, attempt domainexercise.ChallengeAttempt) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if hasRunning {
			if err := s.attempts.DeleteUnfinished(ctx, running.ID); err != nil {
				return fmt.Errorf("exercise: discarding running attempt: %w", err)
			}
		}
		return s.attempts.Create(ctx, attempt)
	})
	if errors.Is(err, domainexercise.ErrAnotherAttemptInProgress) {
		return s.currentConflict(ctx)
	}
	if err != nil {
		return fmt.Errorf("exercise: storing attempt: %w", err)
	}
	return nil
}

// currentConflict names the attempt that won a race. If it is already gone
// the conflict is still real, only its id is unknown.
func (s *AttemptService) currentConflict(ctx context.Context) error {
	winner, err := s.attempts.FindInProgress(ctx)
	if err != nil {
		return &AttemptInProgressError{}
	}
	return &AttemptInProgressError{AttemptID: winner.ID}
}

func viewOf(attempt domainexercise.ChallengeAttempt, challenge domainexercise.Challenge, rubric domainexercise.RubricVersion, submission *domainexercise.Submission) AttemptView {
	return AttemptView{
		Attempt:        attempt,
		ChallengeTitle: challenge.Title,
		ChallengeTopic: challenge.Topic,
		Criteria:       rubric.Criteria,
		Submission:     submission,
	}
}
