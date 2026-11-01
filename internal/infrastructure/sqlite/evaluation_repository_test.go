package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

var evaluationTestTime = time.Date(2026, 11, 1, 10, 30, 0, 123456789, time.UTC)

// newTestEvaluationRepository returns a repository over a database that holds
// a submitted attempt-1 on rubric-1 (six criteria).
func newTestEvaluationRepository(t *testing.T) (*EvaluationRepository, *AttemptRepository, *sql.DB) {
	t.Helper()
	attempts, db := newTestAttemptRepository(t)
	ctx := context.Background()
	require.NoError(t, attempts.Create(ctx, newTestAttempt("attempt-1")))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, attempts.Submit(ctx, submitted, submission))
	return NewEvaluationRepository(db), attempts, db
}

// newTestEvaluation judges every criterion of rubric-1 in rubric order, the
// last one flagged, with text that needs escaping.
func newTestEvaluation(id string) exercise.Evaluation {
	ids := []string{"architecture", "scalability", "consistency", "resilience", "tradeoffs"}
	results := make([]exercise.CriterionResult, 0, 6)
	for i, key := range ids {
		results = append(results, exercise.CriterionResult{
			CriterionID: "rubric-1-base-" + key, Level: i % 5, Justification: "because \"" + key + "\"", Evidence: "box " + key,
		})
	}
	results = append(results, exercise.CriterionResult{
		CriterionID: "rubric-1-cache", Level: 0, Justification: "not shown ünïcode", InsufficientEvidence: true,
	})
	return exercise.Evaluation{
		ID:              id,
		AttemptID:       "attempt-1",
		Kind:            exercise.EvaluationKindDiagram,
		Model:           "vendor/model-1",
		Score:           64,
		Summary:         "A solid design.",
		Strengths:       []string{"clear data flow", "uses a \"queue\""},
		Gaps:            []string{"no failure handling"},
		Recommendations: []string{},
		Results:         results,
		CreatedAt:       evaluationTestTime,
	}
}

func TestEvaluationRepository_CreateThenGetByAttempt_roundTripsEveryField(t *testing.T) {
	// Given a stored evaluation with flagged and judged results
	repo, _, _ := newTestEvaluationRepository(t)
	ctx := context.Background()
	evaluation := newTestEvaluation("evaluation-1")
	require.NoError(t, repo.Create(ctx, evaluation))

	// When reading it back by attempt and kind
	got, err := repo.GetByAttempt(ctx, "attempt-1", exercise.EvaluationKindDiagram)

	// Then every field survives and the results come back in rubric order
	require.NoError(t, err)
	assert.Equal(t, evaluation, got)
}

func TestEvaluationRepository_GetByAttempt_returnsNotFound(t *testing.T) {
	// Given an attempt without an evaluation
	repo, _, _ := newTestEvaluationRepository(t)

	// When reading it
	_, err := repo.GetByAttempt(context.Background(), "attempt-1", exercise.EvaluationKindDiagram)

	// Then it is not found
	assert.ErrorIs(t, err, exercise.ErrEvaluationNotFound)
}

func TestEvaluationRepository_Create_refusesASecondEvaluationOfTheSameKind(t *testing.T) {
	// Given an attempt that already has a DIAGRAM evaluation
	repo, _, db := newTestEvaluationRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestEvaluation("evaluation-1")))

	// When storing another one for the same attempt and kind
	err := repo.Create(ctx, newTestEvaluation("evaluation-2"))

	// Then it fails and the first evaluation is the only one stored
	require.Error(t, err)
	assert.Equal(t, 1, countTableRows(t, db, "evaluations"))
	assert.Equal(t, 6, countTableRows(t, db, "criterion_results"))
}

func TestEvaluationRepository_Create_storesNothingWhenACriterionResultIsRejected(t *testing.T) {
	// Given an evaluation with a result for a criterion that is not in any rubric
	repo, _, db := newTestEvaluationRepository(t)
	evaluation := newTestEvaluation("evaluation-1")
	evaluation.Results[2].CriterionID = "unknown-criterion"

	// When storing it
	err := repo.Create(context.Background(), evaluation)

	// Then it fails and the evaluation row is rolled back with it
	require.Error(t, err)
	assert.Equal(t, 0, countTableRows(t, db, "evaluations"))
	assert.Equal(t, 0, countTableRows(t, db, "criterion_results"))
}
