package exercise

import (
	"context"
	"errors"
	"fmt"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// ProgressService reads the pass/fail outcome of an attempt and the progress
// of a challenge. Both are computed on read from the stored attempts and
// evaluations, never stored. See
// specs/phases/phase-03-architecture-lab/06-completion-rules.md.
type ProgressService struct {
	attempts    domainexercise.AttemptRepository
	rubrics     domainexercise.RubricRepository
	evaluations domainexercise.EvaluationRepository
}

// NewProgressService creates a ProgressService backed by the given ports.
func NewProgressService(
	attempts domainexercise.AttemptRepository,
	rubrics domainexercise.RubricRepository,
	evaluations domainexercise.EvaluationRepository,
) *ProgressService {
	return &ProgressService{attempts: attempts, rubrics: rubrics, evaluations: evaluations}
}

// GetAttemptOutcome returns the attempt's outcome under the minimum passing
// score frozen on it. It is PENDING until both evaluations exist.
func (s *ProgressService) GetAttemptOutcome(ctx context.Context, attemptID string) (domainexercise.AttemptOutcome, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return domainexercise.AttemptOutcome{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	summary, err := s.summarize(ctx, attempt)
	if err != nil {
		return domainexercise.AttemptOutcome{}, err
	}
	return summary.Outcome, nil
}

// GetChallengeProgress derives the challenge's progress from all its attempts.
func (s *ProgressService) GetChallengeProgress(ctx context.Context, challengeID string) (domainexercise.ChallengeProgress, error) {
	attempts, err := s.attempts.ListByChallenge(ctx, challengeID)
	if err != nil {
		return domainexercise.ChallengeProgress{}, fmt.Errorf("exercise: listing attempts: %w", err)
	}
	summaries := make([]domainexercise.AttemptSummary, 0, len(attempts))
	for _, attempt := range attempts {
		summary, err := s.summarize(ctx, attempt)
		if err != nil {
			return domainexercise.ChallengeProgress{}, err
		}
		summaries = append(summaries, summary)
	}
	return domainexercise.DeriveProgress(summaries), nil
}

// summarize reads what the attempt's evaluations say. A running attempt has
// none, so nothing is read for it.
func (s *ProgressService) summarize(ctx context.Context, attempt domainexercise.ChallengeAttempt) (domainexercise.AttemptSummary, error) {
	pending := domainexercise.AttemptOutcome{Result: domainexercise.ResultPending}
	if attempt.State == domainexercise.AttemptInProgress {
		return domainexercise.AttemptSummary{Running: true, Outcome: pending}, nil
	}
	diagram, err := s.findEvaluation(ctx, attempt.ID, domainexercise.EvaluationKindDiagram)
	if err != nil {
		return domainexercise.AttemptSummary{}, err
	}
	understanding, err := s.findEvaluation(ctx, attempt.ID, domainexercise.EvaluationKindUnderstanding)
	if err != nil {
		return domainexercise.AttemptSummary{}, err
	}
	summary := domainexercise.AttemptSummary{Outcome: pending, SubmittedAt: attempt.SubmittedAt}
	if diagram != nil {
		summary.DiagramScore = diagram.Score
	}
	if diagram == nil || understanding == nil {
		return summary, nil
	}
	if err := domainexercise.ValidatePassScore(attempt.PassScore); err != nil {
		return domainexercise.AttemptSummary{}, fmt.Errorf("exercise: attempt %s has no valid frozen pass score: %w", attempt.ID, err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return domainexercise.AttemptSummary{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	summary.Outcome = domainexercise.EvaluateOutcome(attempt.PassScore, rubric, diagram, understanding)
	return summary, nil
}

// findEvaluation returns the attempt's evaluation of the kind, or nil when it
// has none yet.
func (s *ProgressService) findEvaluation(ctx context.Context, attemptID string, kind domainexercise.EvaluationKind) (*domainexercise.Evaluation, error) {
	evaluation, err := s.evaluations.GetByAttempt(ctx, attemptID, kind)
	if errors.Is(err, domainexercise.ErrEvaluationNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exercise: getting %s evaluation: %w", kind, err)
	}
	return &evaluation, nil
}
