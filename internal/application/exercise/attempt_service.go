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

	attempt, err := domainexercise.NewAttempt(challenge, rubric, "", s.clock.Now())
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: creating attempt: %w", err)
	}
	attempt.ID = s.newID()
	if err := s.attempts.Create(ctx, attempt); err != nil {
		return AttemptView{}, fmt.Errorf("exercise: storing attempt: %w", err)
	}
	return viewOf(attempt, challenge, rubric, nil), nil
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
