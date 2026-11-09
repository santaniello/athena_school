package desktop

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

type fakeDefense struct {
	turn       domainexercise.DefenseTurn
	view       applicationexercise.DefenseView
	outcome    applicationexercise.DefenseOutcome
	evaluated  domainexercise.Evaluation
	err        error
	outcomeErr error
	readErr    error

	asked, answered, evaluatedID, read, outcomeID string
	answer                                        string
}

func (f *fakeDefense) NextQuestion(_ context.Context, attemptID string) (domainexercise.DefenseTurn, error) {
	f.asked = attemptID
	return f.turn, f.err
}

func (f *fakeDefense) Answer(_ context.Context, attemptID, answer string) (applicationexercise.DefenseView, error) {
	f.answered, f.answer = attemptID, answer
	return f.view, f.err
}

func (f *fakeDefense) Evaluate(_ context.Context, attemptID string) (domainexercise.Evaluation, error) {
	f.evaluatedID = attemptID
	return f.evaluated, f.err
}

func (f *fakeDefense) GetDefense(_ context.Context, attemptID string) (applicationexercise.DefenseView, error) {
	f.read = attemptID
	if f.readErr != nil {
		return applicationexercise.DefenseView{}, f.readErr
	}
	return f.view, f.err
}

func (f *fakeDefense) Outcome(_ context.Context, attemptID string) (applicationexercise.DefenseOutcome, error) {
	f.outcomeID = attemptID
	return f.outcome, f.outcomeErr
}

func newDefenseApp(defense defenseRunner) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithDefenseService(defense))
	app.ctx = context.Background()
	return app
}

var defenseBindingTime = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

func storedDefense() *fakeDefense {
	asked := domainexercise.DefenseTurn{
		ID: "turn-1", AttemptID: "attempt-1", Position: 1, CriterionID: "c1", Question: "Where does the cache live?",
		AskedAt: defenseBindingTime,
	}
	answered := asked
	answered.Position, answered.Question = 2, "What if it fails?"
	answered.Answer, answered.AnsweredAt = "We fall back.", defenseBindingTime.Add(time.Minute)
	return &fakeDefense{
		turn: asked,
		view: applicationexercise.DefenseView{
			Total: 5, Turns: []domainexercise.DefenseTurn{asked, answered}, Complete: false,
		},
		outcome: applicationexercise.DefenseOutcome{
			Diagram:       domainexercise.Evaluation{Score: 85},
			Understanding: domainexercise.Evaluation{Score: 60},
			Criteria: []domainexercise.RubricCriterion{
				{ID: "c1", Key: "architecture", Name: "Architecture"}, {ID: "c2", Key: "scalability", Name: "Scalability"},
			},
			PassScore: 80,
			Outcome: domainexercise.AttemptOutcome{
				Result: domainexercise.ResultFailed,
				Reasons: []domainexercise.FailureReason{
					{Kind: domainexercise.FailureUnderstandingScore, Evaluation: domainexercise.EvaluationKindUnderstanding},
					{Kind: domainexercise.FailureMandatoryCriterion, Evaluation: domainexercise.EvaluationKindDiagram, CriterionID: "c2"},
				},
			},
		},
	}
}

func TestApp_NextDefenseQuestion_returnsTheQuestionWithItsPositionAndTheTotal(t *testing.T) {
	// Given a service that asks, in a defense of five questions
	defense := storedDefense()
	app := newDefenseApp(defense)

	// When the frontend asks for the next question with a padded id
	got, err := app.NextDefenseQuestion(" attempt-1 ")

	// Then position, text and total are exposed, and nothing hints at the criterion
	require.NoError(t, err)
	assert.Equal(t, DefenseQuestionDTO{Position: 1, Question: "Where does the cache live?", Total: 5}, got)
	assert.Equal(t, "attempt-1", defense.asked)
	assert.Equal(t, "attempt-1", defense.read)
}

func TestApp_NextDefenseQuestion_reportsAFailureToReadTheTotal(t *testing.T) {
	// Given a question that was asked but a defense that cannot be read
	defense := storedDefense()
	defense.readErr = domainexercise.ErrDefenseNotActive
	app := newDefenseApp(defense)

	// When asking
	_, err := app.NextDefenseQuestion("attempt-1")

	// Then the failure is coded
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeDefenseNotActive, exerciseErr.Code)
}

func TestApp_AnswerDefenseQuestion_returnsTheDefenseAfterTheAnswer(t *testing.T) {
	// Given a service that stores answers
	defense := storedDefense()
	app := newDefenseApp(defense)

	// When the frontend answers
	got, err := app.AnswerDefenseQuestion(" attempt-1 ", "We fall back.")

	// Then the answer reaches the service as typed and the defense comes back
	require.NoError(t, err)
	assert.Equal(t, "attempt-1", defense.answered)
	assert.Equal(t, "We fall back.", defense.answer)
	assert.Equal(t, expectedDefenseDTO(), got)
}

func expectedDefenseDTO() DefenseDTO {
	return DefenseDTO{
		Total: 5, Complete: false,
		Turns: []DefenseTurnDTO{
			{Position: 1, Question: "Where does the cache live?", AskedAt: "2026-11-07T10:00:00Z"},
			{Position: 2, Question: "What if it fails?", Answer: "We fall back.", AskedAt: "2026-11-07T10:00:00Z", AnsweredAt: "2026-11-07T10:01:00Z"},
		},
	}
}

func TestApp_GetDefense_returnsTheTranscript(t *testing.T) {
	// Given a stored defense
	defense := storedDefense()
	app := newDefenseApp(defense)

	// When reading it
	got, err := app.GetDefense(" attempt-1 ")

	// Then it is mapped with an empty list, never null, when there are no turns
	require.NoError(t, err)
	assert.Equal(t, expectedDefenseDTO(), got)
	assert.Equal(t, "attempt-1", defense.read)

	defense.view = applicationexercise.DefenseView{Total: 5}
	empty, err := app.GetDefense("attempt-1")
	require.NoError(t, err)
	assert.Equal(t, DefenseDTO{Total: 5, Turns: []DefenseTurnDTO{}}, empty)
}

func TestApp_EvaluateDefense_returnsTheOutcomeWithBothScoresAndTheReasons(t *testing.T) {
	// Given a service whose evaluation finishes the attempt as failed
	defense := storedDefense()
	app := newDefenseApp(defense)

	// When the frontend evaluates the defense
	got, err := app.EvaluateDefense(" attempt-1 ")

	// Then the outcome has the result, both scores, the pass score and a reason per failure with criterion names
	require.NoError(t, err)
	assert.Equal(t, "attempt-1", defense.evaluatedID)
	assert.Equal(t, "attempt-1", defense.outcomeID)
	assert.Equal(t, OutcomeDTO{
		Result: "FAILED", PassScore: 80, DiagramScore: 85, UnderstandingScore: 60,
		Reasons: []OutcomeReasonDTO{
			{Kind: "UNDERSTANDING_SCORE", Evaluation: "UNDERSTANDING"},
			{Kind: "MANDATORY_CRITERION", Evaluation: "DIAGRAM", CriterionKey: "scalability", CriterionName: "Scalability"},
		},
	}, got)
}

func TestApp_EvaluateDefense_aPassedAttemptHasNoReasons(t *testing.T) {
	// Given an attempt that passed
	defense := storedDefense()
	defense.outcome.Outcome = domainexercise.AttemptOutcome{Result: domainexercise.ResultPassed}
	app := newDefenseApp(defense)

	// When evaluating
	got, err := app.EvaluateDefense("attempt-1")

	// Then the reasons are an empty list, never null
	require.NoError(t, err)
	assert.Equal(t, "PASSED", got.Result)
	assert.Equal(t, []OutcomeReasonDTO{}, got.Reasons)
}

func TestApp_EvaluateDefense_anOutcomeReadFailureIsReported(t *testing.T) {
	// Given an evaluation that works but an outcome that cannot be read
	defense := storedDefense()
	defense.outcomeErr = domainexercise.ErrEvaluationNotFound
	app := newDefenseApp(defense)

	// When evaluating
	_, err := app.EvaluateDefense("attempt-1")

	// Then the failure is coded
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeEvaluationNotFound, exerciseErr.Code)
}

func TestApp_defenseBindings_rejectABlankAttemptId(t *testing.T) {
	// Given a service that must not be called
	defense := &fakeDefense{}
	app := newDefenseApp(defense)

	// When asking with a blank id
	_, askErr := app.NextDefenseQuestion(" ")
	_, answerErr := app.AnswerDefenseQuestion(" ", "x")
	_, evaluateErr := app.EvaluateDefense(" ")
	_, getErr := app.GetDefense(" ")

	// Then every one is invalid input and the service is untouched
	for _, err := range []error{askErr, answerErr, evaluateErr, getErr} {
		var exerciseErr *ExerciseError
		require.ErrorAs(t, err, &exerciseErr)
		assert.Equal(t, exerciseCodeInvalidInput, exerciseErr.Code)
	}
	assert.Empty(t, defense.asked+defense.answered+defense.evaluatedID+defense.read)
}

func TestApp_defenseBindings_mapEachFailureToItsCode(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{domainexercise.ErrDefenseNotActive, exerciseCodeDefenseNotActive},
		{domainexercise.ErrQuestionPending, exerciseCodeQuestionPending},
		{domainexercise.ErrNoQuestionPending, exerciseCodeNoQuestionPending},
		{domainexercise.ErrInvalidAnswer, exerciseCodeInvalidAnswer},
		{applicationexercise.ErrEmptyQuestion, exerciseCodeEmptyQuestion},
		{fmt.Errorf("x: %w", domainexercise.ErrInvalidEvaluation), exerciseCodeInvalidEvaluation},
		{domainexercise.ErrAttemptNotFound, exerciseCodeAttemptNotFound},
		{context.Canceled, exerciseCodeCancelled},
		{errors.New("provider down"), exerciseCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			// Given a service that fails
			app := newDefenseApp(&fakeDefense{err: tt.err})

			// When using every binding
			_, askErr := app.NextDefenseQuestion("attempt-1")
			_, answerErr := app.AnswerDefenseQuestion("attempt-1", "x")
			_, evaluateErr := app.EvaluateDefense("attempt-1")
			_, getErr := app.GetDefense("attempt-1")

			// Then each returns the coded error
			for _, err := range []error{askErr, answerErr, evaluateErr, getErr} {
				var exerciseErr *ExerciseError
				require.ErrorAs(t, err, &exerciseErr)
				assert.Equal(t, tt.code, exerciseErr.Code)
			}
		})
	}
}
