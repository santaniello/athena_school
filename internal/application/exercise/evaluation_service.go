package exercise

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// Short, user-facing reasons stored on an attempt whose evaluation failed.
// The raw error is returned to the caller, never stored. An evaluator error
// may bring its own short reason through a FailureReason method.
const (
	reasonEvaluatorFailed = "The evaluator could not complete the evaluation."
	reasonInvalidOutput   = "The evaluator returned an invalid evaluation."
	reasonCancelled       = "The evaluation was cancelled."
	reasonNotSaved        = "The evaluation could not be saved."
	reasonPassScore       = "The minimum passing score could not be verified."
)

// EvaluationService scores a submitted attempt against its frozen rubric. The
// evaluator picks levels, the domain computes the score, and a successful
// evaluation is final. A failure leaves the attempt EVALUATION_FAILED for the
// user to retry. See
// specs/phases/phase-03-architecture-lab/05-evaluation-engine.md.
type EvaluationService struct {
	challenges  domainexercise.ChallengeRepository
	rubrics     domainexercise.RubricRepository
	attempts    domainexercise.AttemptRepository
	submissions domainexercise.SubmissionRepository
	evaluations domainexercise.EvaluationRepository
	evaluator   domainexercise.Evaluator
	tx          Transactor
	passScores  PassScoreProvider
	clock       Clock
	newID       func() string
}

// NewEvaluationService creates an EvaluationService backed by the given ports.
func NewEvaluationService(
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	submissions domainexercise.SubmissionRepository,
	evaluations domainexercise.EvaluationRepository,
	evaluator domainexercise.Evaluator,
	tx Transactor,
	passScores PassScoreProvider,
	clock Clock,
) *EvaluationService {
	return &EvaluationService{
		challenges:  challenges,
		rubrics:     rubrics,
		attempts:    attempts,
		submissions: submissions,
		evaluations: evaluations,
		evaluator:   evaluator,
		tx:          tx,
		passScores:  passScores,
		clock:       clock,
		newID:       uuid.NewString,
	}
}

// Evaluate scores the attempt. It runs from SUBMITTED or EVALUATION_FAILED; an
// EVALUATED attempt returns its stored evaluation without calling the
// evaluator, and any other state, or a concurrent evaluation already running,
// is refused with ErrAttemptNotEvaluable. If the evaluator fails, returns
// invalid output or the context is cancelled, the attempt becomes
// EVALUATION_FAILED and the cause is returned.
func (s *EvaluationService) Evaluate(ctx context.Context, attemptID string) (domainexercise.Evaluation, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	if attempt.State == domainexercise.AttemptEvaluated {
		return s.GetEvaluation(ctx, attemptID)
	}
	if _, err := attempt.StartEvaluation(); err != nil {
		return domainexercise.Evaluation{}, err
	}
	// Everything the evaluator needs is read before the attempt leaves its
	// current state, so a failed read cannot strand it in EVALUATING.
	input, err := s.evaluatorInput(ctx, attempt)
	if err != nil {
		return domainexercise.Evaluation{}, err
	}
	if err := s.attempts.StartEvaluation(ctx, attemptID); err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: starting evaluation: %w", err)
	}

	output, err := s.evaluator.Evaluate(ctx, input)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return domainexercise.Evaluation{}, s.failEvaluation(ctx, attemptID, err)
	}
	evaluation, err := domainexercise.NewEvaluation(attempt, input.Rubric, output, output.Model, s.clock.Now())
	if err != nil {
		return domainexercise.Evaluation{}, s.failEvaluation(ctx, attemptID, err)
	}
	evaluation.ID = s.newID()
	passScore, err := s.currentPassScore(ctx)
	if err != nil {
		return domainexercise.Evaluation{}, s.failEvaluation(ctx, attemptID, err)
	}
	if err := s.storeEvaluation(ctx, evaluation, passScore); err != nil {
		return domainexercise.Evaluation{}, s.failEvaluation(ctx, attemptID, fmt.Errorf("%w: %w", errNotSaved, err))
	}
	return evaluation, nil
}

// errPassScoreUnverified marks a minimum passing score that could not be read
// or is invalid, so no default is ever frozen in its place.
var errPassScoreUnverified = errors.New("exercise: verifying the minimum passing score")

func (s *EvaluationService) currentPassScore(ctx context.Context) (int, error) {
	passScore, err := s.passScores.Current(ctx)
	if err == nil {
		err = domainexercise.ValidatePassScore(passScore)
	}
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errPassScoreUnverified, err)
	}
	return passScore, nil
}

// errNotSaved marks a failure to store a valid evaluation, so it gets its own reason.
var errNotSaved = errors.New("exercise: storing evaluation")

func (s *EvaluationService) evaluatorInput(ctx context.Context, attempt domainexercise.ChallengeAttempt) (domainexercise.EvaluatorInput, error) {
	challenge, err := s.challenges.Get(ctx, attempt.ChallengeID)
	if err != nil {
		return domainexercise.EvaluatorInput{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return domainexercise.EvaluatorInput{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	submission, err := s.submissions.GetByAttempt(ctx, attempt.ID)
	if err != nil {
		return domainexercise.EvaluatorInput{}, fmt.Errorf("exercise: getting submission: %w", err)
	}
	return domainexercise.EvaluatorInput{Challenge: challenge, Rubric: rubric, Submission: submission}, nil
}

// storeEvaluation writes the evaluation and the EVALUATED state, with the
// minimum passing score frozen on the attempt, in one transaction.
func (s *EvaluationService) storeEvaluation(ctx context.Context, evaluation domainexercise.Evaluation, passScore int) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.evaluations.Create(ctx, evaluation); err != nil {
			return err
		}
		return s.attempts.CompleteEvaluation(ctx, evaluation.AttemptID, passScore)
	})
}

// failEvaluation moves the attempt to EVALUATION_FAILED and returns cause. The
// write uses a context that outlives a cancellation, since a cancelled
// evaluation is exactly when it is needed. If the write itself fails, both
// errors are returned.
func (s *EvaluationService) failEvaluation(ctx context.Context, attemptID string, cause error) error {
	failErr := s.attempts.FailEvaluation(context.WithoutCancel(ctx), attemptID, failureReason(cause))
	if failErr != nil {
		return errors.Join(cause, fmt.Errorf("exercise: recording evaluation failure: %w", failErr))
	}
	return cause
}

func failureReason(cause error) string {
	switch {
	case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
		return reasonCancelled
	case errors.Is(cause, domainexercise.ErrInvalidEvaluation):
		return reasonInvalidOutput
	case errors.Is(cause, errNotSaved):
		return reasonNotSaved
	case errors.Is(cause, errPassScoreUnverified):
		return reasonPassScore
	}
	var reasoned interface{ FailureReason() string }
	if errors.As(cause, &reasoned) {
		return reasoned.FailureReason()
	}
	return reasonEvaluatorFailed
}

// GetEvaluation returns the attempt's stored diagram evaluation, or
// ErrEvaluationNotFound.
func (s *EvaluationService) GetEvaluation(ctx context.Context, attemptID string) (domainexercise.Evaluation, error) {
	evaluation, err := s.evaluations.GetByAttempt(ctx, attemptID, domainexercise.EvaluationKindDiagram)
	if err != nil {
		return domainexercise.Evaluation{}, fmt.Errorf("exercise: getting evaluation: %w", err)
	}
	return evaluation, nil
}

// Criteria returns the criteria of the rubric version the attempt froze, so a
// stored evaluation's results can be shown with their names.
func (s *EvaluationService) Criteria(ctx context.Context, attemptID string) ([]domainexercise.RubricCriterion, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return nil, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return nil, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	return rubric.Criteria, nil
}

// DiscardUnfinishedEvaluations deletes every attempt left in EVALUATING or
// EVALUATION_FAILED, with its submission, and returns how many it removed. The
// exam is one sitting, so an unfinished evaluation is never kept. The caller
// logs a failure instead of stopping the app.
func (s *EvaluationService) DiscardUnfinishedEvaluations(ctx context.Context) (int, error) {
	removed, err := s.attempts.DeleteUnfinishedEvaluations(ctx)
	if err != nil {
		return 0, fmt.Errorf("exercise: discarding unfinished evaluations: %w", err)
	}
	return removed, nil
}
