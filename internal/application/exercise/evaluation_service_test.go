package exercise

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type evaluationFixture struct {
	service     *EvaluationService
	challenges  *domainmocks.MockChallengeRepository
	rubrics     *domainmocks.MockRubricRepository
	attempts    *domainmocks.MockAttemptRepository
	submissions *domainmocks.MockSubmissionRepository
	evaluations *domainmocks.MockEvaluationRepository
	evaluator   *domainmocks.MockEvaluator
	tx          *exercisemocks.MockTransactor
}

func newEvaluationFixture(t *testing.T) *evaluationFixture {
	t.Helper()
	f := &evaluationFixture{
		challenges:  domainmocks.NewMockChallengeRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		attempts:    domainmocks.NewMockAttemptRepository(t),
		submissions: domainmocks.NewMockSubmissionRepository(t),
		evaluations: domainmocks.NewMockEvaluationRepository(t),
		evaluator:   domainmocks.NewMockEvaluator(t),
		tx:          exercisemocks.NewMockTransactor(t),
	}
	f.service = NewEvaluationService(f.challenges, f.rubrics, f.attempts, f.submissions, f.evaluations, f.evaluator, f.tx,
		&fakeClock{now: fixedNow})
	f.service.newID = func() string { return "evaluation-1" }
	return f
}

// evaluationRubric has two criteria of weight 50.
func evaluationRubric() domainexercise.RubricVersion {
	return domainexercise.RubricVersion{
		ID: "rubric-2", ChallengeID: "challenge-1", Version: 2,
		Criteria: []domainexercise.RubricCriterion{{ID: "c1", Weight: 50}, {ID: "c2", Weight: 50}},
	}
}

func attemptToEvaluate(state domainexercise.AttemptState) domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-2", State: state,
	}
}

func evaluatorOutput() domainexercise.EvaluatorOutput {
	return domainexercise.EvaluatorOutput{
		Model:           "vendor/model-1",
		Summary:         "A solid design.",
		Strengths:       []string{"clear data flow"},
		Gaps:            []string{"no failure handling"},
		Recommendations: []string{"add a queue"},
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", Level: 4, Justification: "complete"},
			{CriterionID: "c2", Level: 2, Justification: "partial"},
		},
	}
}

var evaluationSubmission = domainexercise.Submission{ID: "submission-1", AttemptID: "attempt-1", ContentType: "application/json", Content: "{}"}

// expectLoad makes the service read what the evaluator needs.
func (f *evaluationFixture) expectLoad(ctx context.Context, state domainexercise.AttemptState) {
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attemptToEvaluate(state), nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(evaluationRubric(), nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(evaluationSubmission, nil)
}

func (f *evaluationFixture) expectEvaluatorInput() domainexercise.EvaluatorInput {
	return domainexercise.EvaluatorInput{Challenge: attemptChallenge(), Rubric: evaluationRubric(), Submission: evaluationSubmission}
}

func (f *evaluationFixture) runTx(ctx context.Context) {
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

// liveContext matches the context the failure is written with: derived from
// the caller's but not cancelled with it.
func liveContext() any {
	return mock.MatchedBy(func(c context.Context) bool { return c.Err() == nil })
}

func expectedEvaluation() domainexercise.Evaluation {
	output := evaluatorOutput()
	return domainexercise.Evaluation{
		ID: "evaluation-1", AttemptID: "attempt-1", Kind: domainexercise.EvaluationKindDiagram,
		Model: "vendor/model-1", Score: 75, Summary: output.Summary, Strengths: output.Strengths,
		Gaps: output.Gaps, Recommendations: output.Recommendations, Results: output.Results, CreatedAt: fixedNow,
	}
}

func TestEvaluationService_Evaluate_storesTheEvaluationAndMarksTheAttemptEvaluated(t *testing.T) {
	for _, from := range []domainexercise.AttemptState{domainexercise.AttemptSubmitted, domainexercise.AttemptEvaluationFailed} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt that can be evaluated and an evaluator that returns valid levels
			f := newEvaluationFixture(t)
			ctx := context.Background()
			f.expectLoad(ctx, from)
			f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(nil)
			f.evaluator.EXPECT().Evaluate(ctx, f.expectEvaluatorInput()).Return(evaluatorOutput(), nil)
			f.runTx(ctx)
			f.evaluations.EXPECT().Create(ctx, expectedEvaluation()).Return(nil)
			f.attempts.EXPECT().CompleteEvaluation(ctx, "attempt-1").Return(nil)

			// When evaluating
			evaluation, err := f.service.Evaluate(ctx, "attempt-1")

			// Then the domain-computed evaluation is stored and returned
			require.NoError(t, err)
			assert.Equal(t, expectedEvaluation(), evaluation)
		})
	}
}

func TestEvaluationService_Evaluate_callsTheEvaluatorOutsideAnyTransaction(t *testing.T) {
	// Given a transaction that is open only while its function runs
	f := newEvaluationFixture(t)
	ctx := context.Background()
	inTx := false
	f.expectLoad(ctx, domainexercise.AttemptSubmitted)
	f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(nil)
	f.evaluator.EXPECT().Evaluate(ctx, f.expectEvaluatorInput()).RunAndReturn(
		func(context.Context, domainexercise.EvaluatorInput) (domainexercise.EvaluatorOutput, error) {
			assert.False(t, inTx, "the evaluator ran inside a transaction")
			return evaluatorOutput(), nil
		})
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			inTx = true
			defer func() { inTx = false }()
			return fn(ctx)
		})
	f.evaluations.EXPECT().Create(ctx, expectedEvaluation()).Return(nil)
	f.attempts.EXPECT().CompleteEvaluation(ctx, "attempt-1").Return(nil)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then it succeeds without the evaluator ever seeing a transaction
	require.NoError(t, err)
}

func TestEvaluationService_Evaluate_returnsTheStoredEvaluationOfAnEvaluatedAttemptWithoutCallingTheEvaluator(t *testing.T) {
	// Given an already evaluated attempt
	f := newEvaluationFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attemptToEvaluate(domainexercise.AttemptEvaluated), nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(expectedEvaluation(), nil)

	// When evaluating again
	evaluation, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the stored evaluation comes back and the evaluator is never called
	require.NoError(t, err)
	assert.Equal(t, expectedEvaluation(), evaluation)
}

func TestEvaluationService_Evaluate_refusesAnAttemptThatCannotBeEvaluated(t *testing.T) {
	for _, from := range []domainexercise.AttemptState{
		domainexercise.AttemptInProgress, domainexercise.AttemptEvaluating, domainexercise.AttemptFinished,
	} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt in a state that cannot start an evaluation
			f := newEvaluationFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attemptToEvaluate(from), nil)

			// When evaluating
			_, err := f.service.Evaluate(ctx, "attempt-1")

			// Then it is refused before anything else happens
			assert.ErrorIs(t, err, domainexercise.ErrAttemptNotEvaluable)
		})
	}
}

func TestEvaluationService_Evaluate_aConcurrentStartIsRefusedWithoutCallingTheEvaluator(t *testing.T) {
	// Given a second call that read SUBMITTED but lost the conditional update
	f := newEvaluationFixture(t)
	ctx := context.Background()
	f.expectLoad(ctx, domainexercise.AttemptSubmitted)
	f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(domainexercise.ErrAttemptNotEvaluable)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then it is refused and the evaluator is never reached
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotEvaluable)
}

func TestEvaluationService_Evaluate_reportsAnUnknownAttempt(t *testing.T) {
	// Given no such attempt
	f := newEvaluationFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When evaluating it
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then it is not found
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
}

func TestEvaluationService_Evaluate_leavesTheAttemptUntouchedWhenWhatTheEvaluatorNeedsCannotBeRead(t *testing.T) {
	loadFailure := errors.New("disk failure")
	tests := []struct {
		name   string
		expect func(f *evaluationFixture, ctx context.Context)
	}{
		{"challenge", func(f *evaluationFixture, ctx context.Context) {
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, loadFailure)
		}},
		{"rubric", func(f *evaluationFixture, ctx context.Context) {
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
			f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(domainexercise.RubricVersion{}, loadFailure)
		}},
		{"submission", func(f *evaluationFixture, ctx context.Context) {
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
			f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(evaluationRubric(), nil)
			f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(domainexercise.Submission{}, loadFailure)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing read
			f := newEvaluationFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attemptToEvaluate(domainexercise.AttemptSubmitted), nil)
			tt.expect(f, ctx)

			// When evaluating
			_, err := f.service.Evaluate(ctx, "attempt-1")

			// Then the error is returned and the attempt never left SUBMITTED (no state call was mocked)
			assert.ErrorIs(t, err, loadFailure)
		})
	}
}

func TestEvaluationService_Evaluate_failsTheAttemptWhenTheEvaluatorFailsOrReturnsInvalidOutput(t *testing.T) {
	invalid := evaluatorOutput()
	invalid.Results = invalid.Results[:1]
	evaluatorFailure := errors.New("provider down")
	tests := []struct {
		name       string
		output     domainexercise.EvaluatorOutput
		err        error
		wantErr    error
		wantReason string
	}{
		{"evaluator error", domainexercise.EvaluatorOutput{}, evaluatorFailure, evaluatorFailure, "The evaluator could not complete the evaluation."},
		{"invalid output", invalid, nil, domainexercise.ErrInvalidEvaluation, "The evaluator returned an invalid evaluation."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an evaluator that fails or returns output that breaks the rubric
			f := newEvaluationFixture(t)
			ctx := context.Background()
			f.expectLoad(ctx, domainexercise.AttemptSubmitted)
			f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(nil)
			f.evaluator.EXPECT().Evaluate(ctx, f.expectEvaluatorInput()).Return(tt.output, tt.err)
			f.attempts.EXPECT().FailEvaluation(liveContext(), "attempt-1", tt.wantReason).Return(nil)

			// When evaluating
			_, err := f.service.Evaluate(ctx, "attempt-1")

			// Then the attempt is EVALUATION_FAILED with a short reason, nothing is stored and the cause is returned
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestEvaluationService_Evaluate_aCancelledContextFailsTheAttempt(t *testing.T) {
	tests := []struct {
		name      string
		evaluator func(cancel context.CancelFunc) (domainexercise.EvaluatorOutput, error)
	}{
		{"evaluator returns the cancellation", func(cancel context.CancelFunc) (domainexercise.EvaluatorOutput, error) {
			cancel()
			return domainexercise.EvaluatorOutput{}, context.Canceled
		}},
		{"evaluator ignores it and succeeds", func(cancel context.CancelFunc) (domainexercise.EvaluatorOutput, error) {
			cancel()
			return evaluatorOutput(), nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a context cancelled while the evaluator runs
			f := newEvaluationFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.expectLoad(ctx, domainexercise.AttemptSubmitted)
			f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(nil)
			f.evaluator.EXPECT().Evaluate(ctx, f.expectEvaluatorInput()).RunAndReturn(
				func(context.Context, domainexercise.EvaluatorInput) (domainexercise.EvaluatorOutput, error) {
					return tt.evaluator(cancel)
				})
			// The failure is recorded on a context that survives the cancellation.
			f.attempts.EXPECT().FailEvaluation(liveContext(),
				"attempt-1", "The evaluation was cancelled.").Return(nil)

			// When evaluating
			_, err := f.service.Evaluate(ctx, "attempt-1")

			// Then it fails as cancelled and the attempt is EVALUATION_FAILED
			assert.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestEvaluationService_Evaluate_failsTheAttemptWhenTheEvaluationCannotBeStored(t *testing.T) {
	// Given a transaction that fails after a valid evaluation
	f := newEvaluationFixture(t)
	ctx := context.Background()
	storeFailure := errors.New("disk full")
	f.expectLoad(ctx, domainexercise.AttemptSubmitted)
	f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(nil)
	f.evaluator.EXPECT().Evaluate(ctx, f.expectEvaluatorInput()).Return(evaluatorOutput(), nil)
	f.runTx(ctx)
	f.evaluations.EXPECT().Create(ctx, expectedEvaluation()).Return(storeFailure)
	f.attempts.EXPECT().FailEvaluation(liveContext(), "attempt-1", "The evaluation could not be saved.").Return(nil)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the attempt does not stay stuck in EVALUATING and the error is returned
	assert.ErrorIs(t, err, storeFailure)
}

func TestEvaluationService_Evaluate_reportsBothErrorsWhenTheFailureCannotBeRecorded(t *testing.T) {
	// Given an evaluator error and a failure that cannot be written
	f := newEvaluationFixture(t)
	ctx := context.Background()
	evaluatorFailure := errors.New("provider down")
	recordFailure := errors.New("database locked")
	f.expectLoad(ctx, domainexercise.AttemptSubmitted)
	f.attempts.EXPECT().StartEvaluation(ctx, "attempt-1").Return(nil)
	f.evaluator.EXPECT().Evaluate(ctx, f.expectEvaluatorInput()).Return(domainexercise.EvaluatorOutput{}, evaluatorFailure)
	f.attempts.EXPECT().FailEvaluation(liveContext(), "attempt-1", "The evaluator could not complete the evaluation.").Return(recordFailure)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then neither error is lost
	assert.ErrorIs(t, err, evaluatorFailure)
	assert.ErrorIs(t, err, recordFailure)
}

func TestEvaluationService_GetEvaluation_returnsTheStoredEvaluation(t *testing.T) {
	// Given a stored evaluation
	f := newEvaluationFixture(t)
	ctx := context.Background()
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(expectedEvaluation(), nil)

	// When reading it
	evaluation, err := f.service.GetEvaluation(ctx, "attempt-1")

	// Then it is returned
	require.NoError(t, err)
	assert.Equal(t, expectedEvaluation(), evaluation)
}

func TestEvaluationService_GetEvaluation_reportsAMissingEvaluation(t *testing.T) {
	// Given an attempt without an evaluation
	f := newEvaluationFixture(t)
	ctx := context.Background()
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).
		Return(domainexercise.Evaluation{}, domainexercise.ErrEvaluationNotFound)

	// When reading it
	_, err := f.service.GetEvaluation(ctx, "attempt-1")

	// Then it is not found
	assert.ErrorIs(t, err, domainexercise.ErrEvaluationNotFound)
}

func TestEvaluationService_DiscardUnfinishedEvaluations_returnsHowManyWereRemoved(t *testing.T) {
	// Given two attempts left in an unfinished evaluation
	f := newEvaluationFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().DeleteUnfinishedEvaluations(ctx).Return(2, nil)

	// When discarding them
	removed, err := f.service.DiscardUnfinishedEvaluations(ctx)

	// Then the count is returned
	require.NoError(t, err)
	assert.Equal(t, 2, removed)
}

func TestEvaluationService_DiscardUnfinishedEvaluations_returnsTheStoreFailure(t *testing.T) {
	// Given a failing store
	f := newEvaluationFixture(t)
	ctx := context.Background()
	failure := errors.New("disk failure")
	f.attempts.EXPECT().DeleteUnfinishedEvaluations(ctx).Return(0, failure)

	// When discarding
	removed, err := f.service.DiscardUnfinishedEvaluations(ctx)

	// Then the failure is returned for the caller to log
	assert.ErrorIs(t, err, failure)
	assert.Zero(t, removed)
}
