package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

type fakeHistory struct {
	items  []applicationexercise.AttemptListItem
	result applicationexercise.AttemptResultView
	err    error

	listed, read string
}

func (f *fakeHistory) ListHistory(_ context.Context, challengeID string) ([]applicationexercise.AttemptListItem, error) {
	f.listed = challengeID
	return f.items, f.err
}

func (f *fakeHistory) GetAttemptResult(_ context.Context, attemptID string) (applicationexercise.AttemptResultView, error) {
	f.read = attemptID
	return f.result, f.err
}

func newHistoryApp(history historyReader) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithHistoryService(history))
	app.ctx = context.Background()
	return app
}

func TestApp_ListAttemptHistory_returnsTheFinishedAttemptsAsListed(t *testing.T) {
	// Given a history with one attempt
	history := &fakeHistory{items: []applicationexercise.AttemptListItem{{
		AttemptID: "attempt-1", FinishedAt: defenseBindingTime, DiagramScore: 85, UnderstandingScore: 60,
		Outcome: domainexercise.ResultFailed,
	}}}
	app := newHistoryApp(history)

	// When the frontend lists it with a padded id
	got, err := app.ListAttemptHistory(" challenge-1 ")

	// Then each item carries date, scores and outcome
	require.NoError(t, err)
	assert.Equal(t, "challenge-1", history.listed)
	assert.Equal(t, []AttemptHistoryItemDTO{{
		AttemptID: "attempt-1", FinishedAt: "2026-11-07T10:00:00Z", DiagramScore: 85, UnderstandingScore: 60, Outcome: "FAILED",
	}}, got)
}

func TestApp_ListAttemptHistory_anEmptyHistoryIsAnEmptyListNotNull(t *testing.T) {
	// Given a challenge with no finished attempt
	app := newHistoryApp(&fakeHistory{})

	// When listing
	got, err := app.ListAttemptHistory("challenge-1")

	// Then the list is empty and not nil
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestApp_ListAttemptHistory_rejectsABlankChallengeId(t *testing.T) {
	// Given a blank id
	app := newHistoryApp(&fakeHistory{})

	// When listing
	_, err := app.ListAttemptHistory("  ")

	// Then it is invalid input
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeInvalidInput, exerciseErr.Code)
}

func TestApp_ListAttemptHistory_codesAFailure(t *testing.T) {
	// Given a service that fails
	app := newHistoryApp(&fakeHistory{err: errors.New("boom")})

	// When listing
	_, err := app.ListAttemptHistory("challenge-1")

	// Then the failure is coded
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeInternal, exerciseErr.Code)
}

func storedResult() applicationexercise.AttemptResultView {
	criteria := []domainexercise.RubricCriterion{
		{ID: "c1", Key: "architecture", Name: "Architecture", Weight: 60},
		{ID: "c2", Key: "scalability", Name: "Scalability", Weight: 40},
	}
	diagram := domainexercise.Evaluation{
		Score: 85, Model: "m1", Summary: "good", Strengths: []string{"s"}, Gaps: []string{"g"}, Recommendations: []string{"r"},
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", Level: 3, Justification: "ok", Evidence: "box A"},
			{CriterionID: "c2", InsufficientEvidence: true},
		},
		CreatedAt: defenseBindingTime,
	}
	understanding := domainexercise.Evaluation{
		Score: 60, Model: "m2", Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", Level: 2, Justification: "fair"},
			{CriterionID: "c2", InsufficientEvidence: true},
		},
		CreatedAt: defenseBindingTime.Add(time.Hour),
	}
	return applicationexercise.AttemptResultView{
		Challenge: applicationexercise.ChallengeSnapshot{
			ID: "challenge-1", Title: "URL shortener", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced,
			Statement: applicationexercise.ChallengeStatement{
				Scenario: "A link service.", FunctionalRequirements: []string{"Shorten"}, NonFunctionalRequirements: []string{"Fast"},
				Constraints: []string{"No DB"}, Objectives: []string{"Cut load"},
			},
		},
		Attempt: domainexercise.ChallengeAttempt{
			ID: "attempt-1", ChallengeID: "challenge-1", StartedAt: defenseBindingTime.Add(-30 * time.Minute),
			SubmittedAt: defenseBindingTime, SubmitReason: domainexercise.SubmitReasonTimeExpired,
		},
		FinishedAt: defenseBindingTime.Add(time.Hour), TimeSpent: 30 * time.Minute, HintsUsed: 2, PassScore: 80,
		Outcome: domainexercise.AttemptOutcome{
			Result: domainexercise.ResultFailed,
			Reasons: []domainexercise.FailureReason{
				{Kind: domainexercise.FailureUnderstandingScore, Evaluation: domainexercise.EvaluationKindUnderstanding},
			},
		},
		Criteria: criteria,
		Diagram: applicationexercise.EvaluationView{Evaluation: diagram, Rows: []applicationexercise.CriterionRow{
			{Criterion: criteria[0], Result: diagram.Results[0]}, {Criterion: criteria[1], Result: diagram.Results[1]},
		}},
		Understanding: applicationexercise.EvaluationView{Evaluation: understanding, Rows: []applicationexercise.CriterionRow{
			{Criterion: criteria[0], Result: understanding.Results[0]}, {Criterion: criteria[1], Result: understanding.Results[1]},
		}},
		Defense: []domainexercise.DefenseTurn{
			{Position: 1, CriterionID: "c1", Question: "Why?", Answer: "Because.", AskedAt: defenseBindingTime, AnsweredAt: defenseBindingTime.Add(time.Minute)},
		},
		Submission: "<mxfile/>",
	}
}

func TestApp_GetAttemptResult_mapsEverythingTheResultViewShows(t *testing.T) {
	// Given a stored result
	history := &fakeHistory{result: storedResult()}
	app := newHistoryApp(history)

	// When the frontend reads it with a padded id
	got, err := app.GetAttemptResult(" attempt-1 ")

	// Then the challenge, attempt facts, outcome, evaluations, transcript and diagram are all there
	require.NoError(t, err)
	assert.Equal(t, "attempt-1", history.read)
	assert.Equal(t, AttemptResultDTO{
		Challenge: ResultChallengeDTO{
			ID: "challenge-1", Title: "URL shortener", Topic: "caching", Difficulty: "ADVANCED",
			Statement: StatementDTO{
				Scenario: "A link service.", FunctionalRequirements: []string{"Shorten"}, NonFunctionalRequirements: []string{"Fast"},
				Constraints: []string{"No DB"}, Objectives: []string{"Cut load"},
			},
		},
		AttemptID: "attempt-1", StartedAt: "2026-11-07T09:30:00Z", SubmittedAt: "2026-11-07T10:00:00Z", FinishedAt: "2026-11-07T11:00:00Z",
		TimeSpentSeconds: 1800, SubmitReason: "TIME_EXPIRED", HintsUsed: 2, MaxHints: 3,
		Outcome: OutcomeDTO{
			Result: "FAILED", PassScore: 80, DiagramScore: 85, UnderstandingScore: 60,
			Reasons: []OutcomeReasonDTO{{Kind: "UNDERSTANDING_SCORE", Evaluation: "UNDERSTANDING"}},
		},
		Diagram: EvaluationDTO{
			Model: "m1", Score: 85, Summary: "good", Strengths: []string{"s"}, Gaps: []string{"g"}, Recommendations: []string{"r"},
			CreatedAt: "2026-11-07T10:00:00Z",
			Results: []EvaluationResultDTO{
				{CriterionKey: "architecture", CriterionName: "Architecture", Weight: 60, Level: 3, Justification: "ok", Evidence: "box A"},
				{CriterionKey: "scalability", CriterionName: "Scalability", Weight: 40, InsufficientEvidence: true},
			},
		},
		Understanding: EvaluationDTO{
			Model: "m2", Score: 60, Strengths: []string{}, Gaps: []string{}, Recommendations: []string{},
			CreatedAt: "2026-11-07T11:00:00Z",
			Results: []EvaluationResultDTO{
				{CriterionKey: "architecture", CriterionName: "Architecture", Weight: 60, Level: 2, Justification: "fair"},
				{CriterionKey: "scalability", CriterionName: "Scalability", Weight: 40, InsufficientEvidence: true},
			},
		},
		Defense: []DefenseTurnDTO{
			{Position: 1, Question: "Why?", Answer: "Because.", AskedAt: "2026-11-07T10:00:00Z", AnsweredAt: "2026-11-07T10:01:00Z"},
		},
		Submission: "<mxfile/>",
	}, got)
}

func TestApp_GetAttemptResult_noTurnsIsAnEmptyList(t *testing.T) {
	// Given a result with no defense turns
	result := storedResult()
	result.Defense = nil
	app := newHistoryApp(&fakeHistory{result: result})

	// When reading it
	got, err := app.GetAttemptResult("attempt-1")

	// Then the transcript is empty and not nil
	require.NoError(t, err)
	assert.NotNil(t, got.Defense)
	assert.Empty(t, got.Defense)
}

func TestApp_GetAttemptResult_rejectsABlankAttemptId(t *testing.T) {
	// Given a blank id
	app := newHistoryApp(&fakeHistory{})

	// When reading
	_, err := app.GetAttemptResult("")

	// Then it is invalid input
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeInvalidInput, exerciseErr.Code)
}

func TestApp_GetAttemptResult_codesAnAttemptThatIsNotFinished(t *testing.T) {
	// Given an attempt still running
	app := newHistoryApp(&fakeHistory{err: applicationexercise.ErrAttemptNotFinished})

	// When reading its result
	_, err := app.GetAttemptResult("attempt-1")

	// Then the code tells the frontend it is not finished
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeAttemptNotFinished, exerciseErr.Code)
}
