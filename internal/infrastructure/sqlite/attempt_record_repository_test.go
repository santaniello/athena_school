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

// recordFixture is a database with challenge-1 (rubric-1) and challenge-3
// (rubric-3), and a challenge-2 that has no attempts.
type recordFixture struct {
	db       *sql.DB
	attempts *AttemptRepository
	records  *AttemptRecordRepository
}

func newRecordFixture(t *testing.T) *recordFixture {
	t.Helper()
	attempts, db := newTestAttemptRepository(t)
	ctx := context.Background()
	challenges := NewChallengeRepository(db)
	rubrics := NewRubricRepository(db)
	for _, id := range []string{"challenge-2", "challenge-3"} {
		require.NoError(t, challenges.Create(ctx, newTestChallenge(id)))
	}
	third := newTestRubricVersion("rubric-3", 1)
	third.ChallengeID = "challenge-3"
	require.NoError(t, rubrics.Create(ctx, third))
	return &recordFixture{db: db, attempts: attempts, records: NewAttemptRecordRepository(db)}
}

// evaluate stores a submitted attempt of the challenge, its evaluation of the
// kind (every criterion at level 3 but the first, at level 1 and flagged) and
// returns the stored attempt.
func (f *recordFixture) evaluate(t *testing.T, attemptID, challengeID, rubricID string, kind exercise.EvaluationKind, score int) {
	t.Helper()
	ctx := context.Background()
	attempt := newTestAttempt(attemptID)
	attempt.ChallengeID = challengeID
	attempt.RubricVersionID = rubricID
	require.NoError(t, f.attempts.Create(ctx, attempt))
	submitted, submission := submitForTest(t, attempt, "submission-"+attemptID)
	require.NoError(t, f.attempts.Submit(ctx, submitted, submission))

	rubric, err := NewRubricRepository(f.db).Get(ctx, rubricID)
	require.NoError(t, err)
	results := make([]exercise.CriterionResult, 0, len(rubric.Criteria))
	for i, criterion := range rubric.Criteria {
		result := exercise.CriterionResult{CriterionID: criterion.ID, Level: 3, Justification: "j"}
		if i == 0 {
			result.Level, result.InsufficientEvidence = 1, true
		}
		results = append(results, result)
	}
	require.NoError(t, NewEvaluationRepository(f.db).Create(ctx, exercise.Evaluation{
		ID: attemptID + "-" + string(kind), AttemptID: attemptID, Kind: kind, Model: "m", Score: score,
		Summary: "s", Strengths: []string{}, Gaps: []string{}, Recommendations: []string{},
		Results: results, CreatedAt: time.Date(2026, 11, 1, 11, 0, 0, 0, time.UTC),
	}))
}

func TestAttemptRecordRepository_ListByChallenges_groupsAttemptsByChallengeOldestFirst(t *testing.T) {
	// Given two attempts of challenge-1, one of challenge-3 and none of challenge-2
	f := newRecordFixture(t)
	ctx := context.Background()
	submittedOlder, submission := submitForTest(t, newTestAttempt("attempt-old"), "submission-old")
	require.NoError(t, f.attempts.Create(ctx, newTestAttempt("attempt-old")))
	require.NoError(t, f.attempts.Submit(ctx, submittedOlder, submission))
	other := newTestAttempt("attempt-other")
	other.ChallengeID, other.RubricVersionID = "challenge-3", "rubric-3"
	submittedOther, otherSubmission := submitForTest(t, other, "submission-other")
	require.NoError(t, f.attempts.Create(ctx, other))
	require.NoError(t, f.attempts.Submit(ctx, submittedOther, otherSubmission))
	newer := newTestAttempt("attempt-new")
	newer.StartedAt = attemptTestStart.Add(time.Hour)
	newer.Deadline = newer.StartedAt.Add(45 * time.Minute)
	require.NoError(t, f.attempts.Create(ctx, newer))

	// When reading the records of challenge-1 and challenge-2
	got, err := f.records.ListByChallenges(ctx, []string{"challenge-1", "challenge-2"})

	// Then only challenge-1 has an entry, its attempts oldest first, with no evaluations
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got["challenge-1"], 2)
	assert.Equal(t, "attempt-old", got["challenge-1"][0].Attempt.ID)
	assert.Equal(t, "attempt-new", got["challenge-1"][1].Attempt.ID)
	for _, record := range got["challenge-1"] {
		assert.Nil(t, record.Diagram)
		assert.Nil(t, record.Understanding)
		assert.Nil(t, record.Rubric)
	}
}

func TestAttemptRecordRepository_ListByChallenges_loadsEvaluationsAndFrozenRubric(t *testing.T) {
	// Given an attempt with a diagram and an understanding evaluation, and one with only a diagram
	f := newRecordFixture(t)
	f.evaluate(t, "attempt-1", "challenge-1", "rubric-1", exercise.EvaluationKindDiagram, 80)
	f.evaluate(t, "attempt-3", "challenge-3", "rubric-3", exercise.EvaluationKindDiagram, 55)
	ctx := context.Background()
	require.NoError(t, NewEvaluationRepository(f.db).Create(ctx, exercise.Evaluation{
		ID: "attempt-1-understanding", AttemptID: "attempt-1", Kind: exercise.EvaluationKindUnderstanding, Model: "m", Score: 70,
		Summary: "s", Strengths: []string{}, Gaps: []string{}, Recommendations: []string{},
		Results:   []exercise.CriterionResult{{CriterionID: "rubric-1-cache", Level: 2, Justification: "j"}},
		CreatedAt: time.Date(2026, 11, 1, 11, 0, 0, 0, time.UTC),
	}))

	// When reading the records of both challenges
	got, err := f.records.ListByChallenges(ctx, []string{"challenge-1", "challenge-3"})

	// Then each record carries its evaluations and the rubric criteria it was frozen to
	require.NoError(t, err)
	first := got["challenge-1"][0]
	require.NotNil(t, first.Diagram)
	assert.Equal(t, 80, first.Diagram.Score)
	assert.Equal(t, exercise.EvaluationKindDiagram, first.Diagram.Kind)
	require.Len(t, first.Diagram.Results, 6)
	assert.Equal(t, "rubric-1-base-architecture", first.Diagram.Results[0].CriterionID)
	assert.Equal(t, 1, first.Diagram.Results[0].Level)
	assert.True(t, first.Diagram.Results[0].InsufficientEvidence)
	assert.Equal(t, 3, first.Diagram.Results[1].Level)
	assert.False(t, first.Diagram.Results[1].InsufficientEvidence)
	require.NotNil(t, first.Understanding)
	assert.Equal(t, 70, first.Understanding.Score)
	require.NotNil(t, first.Rubric)
	assert.Equal(t, "rubric-1", first.Rubric.ID)
	require.Len(t, first.Rubric.Criteria, 6)
	assert.Equal(t, "rubric-1-base-architecture", first.Rubric.Criteria[0].ID)
	assert.Equal(t, 15, first.Rubric.Criteria[0].Weight)
	assert.Equal(t, "rubric-1-cache", first.Rubric.Criteria[5].ID)
	assert.True(t, first.Rubric.Criteria[5].Mandatory)

	second := got["challenge-3"][0]
	require.NotNil(t, second.Diagram)
	assert.Equal(t, 55, second.Diagram.Score)
	assert.Nil(t, second.Understanding)
	require.NotNil(t, second.Rubric)
	assert.Equal(t, "rubric-3", second.Rubric.ID)
}

func TestAttemptRecordRepository_ListByChallenges_returnsAnEmptyMapForNoIDs(t *testing.T) {
	// Given a repository and no challenge ids
	f := newRecordFixture(t)

	// When reading the records
	got, err := f.records.ListByChallenges(context.Background(), nil)

	// Then the result is empty and not nil
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
