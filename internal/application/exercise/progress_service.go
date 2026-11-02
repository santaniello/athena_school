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
// none, so nothing is read for it; the rubric is read only when both
// evaluations exist.
func (s *ProgressService) summarize(ctx context.Context, attempt domainexercise.ChallengeAttempt) (domainexercise.AttemptSummary, error) {
	record := domainexercise.AttemptRecord{Attempt: attempt}
	if attempt.State == domainexercise.AttemptInProgress {
		return summarizeRecord(record)
	}
	var err error
	if record.Diagram, err = s.findEvaluation(ctx, attempt.ID, domainexercise.EvaluationKindDiagram); err != nil {
		return domainexercise.AttemptSummary{}, err
	}
	if record.Understanding, err = s.findEvaluation(ctx, attempt.ID, domainexercise.EvaluationKindUnderstanding); err != nil {
		return domainexercise.AttemptSummary{}, err
	}
	if record.Diagram != nil && record.Understanding != nil && domainexercise.ValidatePassScore(attempt.PassScore) == nil {
		rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
		if err != nil {
			return domainexercise.AttemptSummary{}, fmt.Errorf("exercise: getting rubric: %w", err)
		}
		record.Rubric = &rubric
	}
	return summarizeRecord(record)
}

// summarizeRecord computes what DeriveProgress needs of one attempt from what
// is known of it. Both evaluations must come with the rubric the attempt was
// frozen to and a valid frozen minimum score; a record that has both
// evaluations without them is inconsistent and is refused.
func summarizeRecord(record domainexercise.AttemptRecord) (domainexercise.AttemptSummary, error) {
	attempt := record.Attempt
	pending := domainexercise.AttemptOutcome{Result: domainexercise.ResultPending}
	if attempt.State == domainexercise.AttemptInProgress {
		return domainexercise.AttemptSummary{Running: true, Outcome: pending}, nil
	}
	summary := domainexercise.AttemptSummary{Outcome: pending, SubmittedAt: attempt.SubmittedAt}
	if record.Diagram != nil {
		summary.DiagramScore = record.Diagram.Score
	}
	if record.Diagram == nil || record.Understanding == nil {
		return summary, nil
	}
	if err := domainexercise.ValidatePassScore(attempt.PassScore); err != nil {
		return domainexercise.AttemptSummary{}, fmt.Errorf("exercise: attempt %s has no valid frozen pass score: %w", attempt.ID, err)
	}
	if record.Rubric == nil {
		return domainexercise.AttemptSummary{}, fmt.Errorf("exercise: attempt %s has no rubric to evaluate against", attempt.ID)
	}
	summary.Outcome = domainexercise.EvaluateOutcome(attempt.PassScore, *record.Rubric, record.Diagram, record.Understanding)
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
