package desktop

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationarchitecture "github.com/santaniello/athena/internal/application/architecture"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

type fakeEvaluations struct {
	evaluation  domainexercise.Evaluation
	criteria    []domainexercise.RubricCriterion
	err         error
	criteriaErr error
	evaluated   string
	read        string
}

func (f *fakeEvaluations) Evaluate(_ context.Context, attemptID string) (domainexercise.Evaluation, error) {
	f.evaluated = attemptID
	return f.evaluation, f.err
}

func (f *fakeEvaluations) GetEvaluation(_ context.Context, attemptID string) (domainexercise.Evaluation, error) {
	f.read = attemptID
	return f.evaluation, f.err
}

func (f *fakeEvaluations) Criteria(context.Context, string) ([]domainexercise.RubricCriterion, error) {
	return f.criteria, f.criteriaErr
}

func newEvaluationApp(evaluations attemptEvaluator) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithEvaluationService(evaluations))
	app.ctx = context.Background()
	return app
}

func storedEvaluation() *fakeEvaluations {
	return &fakeEvaluations{
		evaluation: domainexercise.Evaluation{
			AttemptID: "attempt-1", Kind: domainexercise.EvaluationKindDiagram, Model: "vendor/model-x", Score: 72,
			Summary: "Solid.", Strengths: []string{"API"}, CreatedAt: time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC),
			Results: []domainexercise.CriterionResult{
				{CriterionID: "c2", Level: 0, Justification: "Not shown.", InsufficientEvidence: true},
				{CriterionID: "c1", Level: 3, Justification: "Cache.", Evidence: "API -> Cache"},
			},
		},
		criteria: []domainexercise.RubricCriterion{
			{ID: "c1", Key: "scalability", Name: "Scalability"}, {ID: "c2", Key: "resilience", Name: "Resilience"},
		},
	}
}

func expectedEvaluationDTO() EvaluationDTO {
	return EvaluationDTO{
		AttemptID: "attempt-1", Model: "vendor/model-x", Score: 72, Summary: "Solid.",
		Strengths: []string{"API"}, Gaps: []string{}, Recommendations: []string{},
		Results: []EvaluationResultDTO{
			{CriterionKey: "resilience", CriterionName: "Resilience", Level: 0, Justification: "Not shown.", InsufficientEvidence: true},
			{CriterionKey: "scalability", CriterionName: "Scalability", Level: 3, Justification: "Cache.", Evidence: "API -> Cache"},
		},
		CreatedAt: "2026-11-07T10:00:00Z",
	}
}

func TestApp_EvaluateAttempt_returnsTheEvaluation(t *testing.T) {
	// Given a service that evaluates
	evaluations := storedEvaluation()
	app := newEvaluationApp(evaluations)

	// When the frontend evaluates with a padded id
	got, err := app.EvaluateAttempt(" attempt-1 ")

	// Then the DTO carries the score, the model and each result with its criterion name
	require.NoError(t, err)
	assert.Equal(t, expectedEvaluationDTO(), got)
	assert.Equal(t, "attempt-1", evaluations.evaluated)
}

func TestApp_GetEvaluation_returnsTheStoredEvaluation(t *testing.T) {
	// Given a stored evaluation
	evaluations := storedEvaluation()
	app := newEvaluationApp(evaluations)

	// When reading it
	got, err := app.GetEvaluation(" attempt-1 ")

	// Then it is mapped the same way
	require.NoError(t, err)
	assert.Equal(t, expectedEvaluationDTO(), got)
	assert.Equal(t, "attempt-1", evaluations.read)
}

func TestApp_evaluationBindings_rejectABlankAttemptId(t *testing.T) {
	// Given a service that must not be called
	evaluations := &fakeEvaluations{}
	app := newEvaluationApp(evaluations)

	// When asking with a blank id
	_, evaluateErr := app.EvaluateAttempt(" ")
	_, getErr := app.GetEvaluation(" ")

	// Then both are invalid input
	for _, err := range []error{evaluateErr, getErr} {
		var exerciseErr *ExerciseError
		require.ErrorAs(t, err, &exerciseErr)
		assert.Equal(t, exerciseCodeInvalidInput, exerciseErr.Code)
	}
	assert.Empty(t, evaluations.evaluated)
	assert.Empty(t, evaluations.read)
}

func TestApp_evaluationBindings_mapEachFailureToItsCode(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{fmt.Errorf("x: %w", applicationarchitecture.ErrInvalidDiagram), exerciseCodeInvalidDiagram},
		{applicationarchitecture.ErrEmptyDiagram, exerciseCodeEmptyDiagram},
		{fmt.Errorf("x: %w", domainexercise.ErrInvalidEvaluation), exerciseCodeInvalidEvaluation},
		{domainexercise.ErrAttemptNotEvaluable, exerciseCodeAttemptNotEvaluable},
		{domainexercise.ErrEvaluationNotFound, exerciseCodeEvaluationNotFound},
		{domainexercise.ErrAttemptNotFound, exerciseCodeAttemptNotFound},
		{context.Canceled, exerciseCodeCancelled},
		{errors.New("provider down"), exerciseCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			// Given a service that fails
			app := newEvaluationApp(&fakeEvaluations{err: tt.err})

			// When evaluating and reading
			_, evaluateErr := app.EvaluateAttempt("attempt-1")
			_, getErr := app.GetEvaluation("attempt-1")

			// Then each carries the matching code
			for _, err := range []error{evaluateErr, getErr} {
				var exerciseErr *ExerciseError
				require.ErrorAs(t, err, &exerciseErr)
				assert.Equal(t, tt.code, exerciseErr.Code)
			}
		})
	}
}

func TestApp_evaluationBindings_reportACriteriaReadFailure(t *testing.T) {
	// Given an evaluation that exists but whose criteria cannot be read
	evaluations := storedEvaluation()
	evaluations.criteriaErr = domainexercise.ErrAttemptNotFound
	app := newEvaluationApp(evaluations)

	// When evaluating
	_, err := app.EvaluateAttempt("attempt-1")

	// Then the failure is coded, not swallowed
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeAttemptNotFound, exerciseErr.Code)
}
