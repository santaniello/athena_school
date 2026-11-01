package exercise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type progressFixture struct {
	service     *ProgressService
	attempts    *domainmocks.MockAttemptRepository
	rubrics     *domainmocks.MockRubricRepository
	evaluations *domainmocks.MockEvaluationRepository
}

func newProgressFixture(t *testing.T) *progressFixture {
	t.Helper()
	f := &progressFixture{
		attempts:    domainmocks.NewMockAttemptRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		evaluations: domainmocks.NewMockEvaluationRepository(t),
	}
	f.service = NewProgressService(f.attempts, f.rubrics, f.evaluations)
	return f
}

// progressRubric has two criteria, the first one mandatory.
func progressRubric() domainexercise.RubricVersion {
	return domainexercise.RubricVersion{
		ID: "rubric-2", ChallengeID: "challenge-1",
		Criteria: []domainexercise.RubricCriterion{{ID: "c1", Weight: 50, Mandatory: true}, {ID: "c2", Weight: 50}},
	}
}

func progressAttempt(id string, state domainexercise.AttemptState, passScore int) domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: id, ChallengeID: "challenge-1", RubricVersionID: "rubric-2", State: state, PassScore: passScore,
		SubmittedAt: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC),
	}
}

func progressEvaluation(attemptID string, kind domainexercise.EvaluationKind, score, mandatoryLevel int) domainexercise.Evaluation {
	return domainexercise.Evaluation{
		ID: attemptID + string(kind), AttemptID: attemptID, Kind: kind, Score: score,
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", Level: mandatoryLevel, Justification: "j"},
			{CriterionID: "c2", Level: 4, Justification: "j"},
		},
	}
}

// expectEvaluations makes the stored evaluations of the attempt the given ones;
// a nil one is reported as not found.
func (f *progressFixture) expectEvaluations(ctx context.Context, attemptID string, diagram, understanding *domainexercise.Evaluation) {
	for kind, stored := range map[domainexercise.EvaluationKind]*domainexercise.Evaluation{
		domainexercise.EvaluationKindDiagram: diagram, domainexercise.EvaluationKindUnderstanding: understanding,
	} {
		if stored == nil {
			f.evaluations.EXPECT().GetByAttempt(ctx, attemptID, kind).Return(domainexercise.Evaluation{}, domainexercise.ErrEvaluationNotFound)
			continue
		}
		f.evaluations.EXPECT().GetByAttempt(ctx, attemptID, kind).Return(*stored, nil)
	}
}

func ptr(e domainexercise.Evaluation) *domainexercise.Evaluation { return &e }

func TestProgressService_GetAttemptOutcome_isPendingWithoutAnUnderstandingEvaluation(t *testing.T) {
	// Given an evaluated attempt with a high diagram score and no defense yet
	f := newProgressFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptEvaluated, 80), nil)
	f.expectEvaluations(ctx, "attempt-1", ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 99, 4)), nil)

	// When reading its outcome
	outcome, err := f.service.GetAttemptOutcome(ctx, "attempt-1")

	// Then it is PENDING
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ResultPending, outcome.Result)
}

func TestProgressService_GetAttemptOutcome_appliesTheMinimumFrozenOnTheAttempt(t *testing.T) {
	tests := []struct {
		name      string
		passScore int
		want      domainexercise.AttemptResult
	}{
		{"scores meet the frozen minimum", 75, domainexercise.ResultPassed},
		{"scores miss the frozen minimum", 90, domainexercise.ResultFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given both evaluations at 85 and an attempt that froze its own minimum
			f := newProgressFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptFinished, tt.passScore), nil)
			f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(progressRubric(), nil)
			f.expectEvaluations(ctx, "attempt-1",
				ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 85, 3)),
				ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindUnderstanding, 85, 3)))

			// When reading its outcome
			outcome, err := f.service.GetAttemptOutcome(ctx, "attempt-1")

			// Then the setting in force today is irrelevant: the frozen value decides
			require.NoError(t, err)
			assert.Equal(t, tt.want, outcome.Result)
		})
	}
}

func TestProgressService_GetAttemptOutcome_listsWhyAnAttemptFailed(t *testing.T) {
	f := newProgressFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptFinished, 80), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(progressRubric(), nil)
	f.expectEvaluations(ctx, "attempt-1",
		ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 86, 0)),
		ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindUnderstanding, 70, 3)))

	outcome, err := f.service.GetAttemptOutcome(ctx, "attempt-1")

	require.NoError(t, err)
	assert.Equal(t, domainexercise.ResultFailed, outcome.Result)
	assert.Equal(t, []domainexercise.FailureReason{
		{Kind: domainexercise.FailureUnderstandingScore, Evaluation: domainexercise.EvaluationKindUnderstanding},
		{Kind: domainexercise.FailureMandatoryCriterion, Evaluation: domainexercise.EvaluationKindDiagram, CriterionID: "c1"},
	}, outcome.Reasons)
}

func TestProgressService_GetAttemptOutcome_refusesBothEvaluationsWithoutAFrozenMinimum(t *testing.T) {
	// Given an inconsistent attempt with both evaluations but no frozen minimum
	f := newProgressFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptFinished, 0), nil)
	f.expectEvaluations(ctx, "attempt-1",
		ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 10, 0)),
		ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindUnderstanding, 10, 0)))

	// When reading its outcome
	_, err := f.service.GetAttemptOutcome(ctx, "attempt-1")

	// Then it errors instead of passing everything against a minimum of zero
	assert.ErrorIs(t, err, domainexercise.ErrInvalidPassScore)
}

func TestProgressService_GetAttemptOutcome_reportsStoreFailures(t *testing.T) {
	failure := errors.New("disk failure")
	ctx := context.Background()
	both := func(f *progressFixture) {
		f.expectEvaluations(ctx, "attempt-1",
			ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 90, 3)),
			ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindUnderstanding, 90, 3)))
	}
	tests := []struct {
		name   string
		expect func(f *progressFixture)
	}{
		{"attempt", func(f *progressFixture) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, failure)
		}},
		{"diagram evaluation", func(f *progressFixture) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptFinished, 80), nil)
			f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(domainexercise.Evaluation{}, failure)
		}},
		{"understanding evaluation", func(f *progressFixture) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptFinished, 80), nil)
			f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).
				Return(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 90, 3), nil)
			f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindUnderstanding).Return(domainexercise.Evaluation{}, failure)
		}},
		{"rubric", func(f *progressFixture) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(progressAttempt("attempt-1", domainexercise.AttemptFinished, 80), nil)
			both(f)
			f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(domainexercise.RubricVersion{}, failure)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newProgressFixture(t)
			tt.expect(f)

			_, err := f.service.GetAttemptOutcome(ctx, "attempt-1")

			assert.ErrorIs(t, err, failure)
		})
	}
}

func TestProgressService_GetChallengeProgress_derivesTheStatusFromTheStoredAttempts(t *testing.T) {
	// Given a failed attempt, a passed one and one still running
	f := newProgressFixture(t)
	ctx := context.Background()
	failed := progressAttempt("attempt-1", domainexercise.AttemptFinished, 80)
	passed := progressAttempt("attempt-2", domainexercise.AttemptFinished, 80)
	passed.SubmittedAt = failed.SubmittedAt.Add(24 * time.Hour)
	running := progressAttempt("attempt-3", domainexercise.AttemptInProgress, 0)
	running.SubmittedAt = time.Time{}
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return([]domainexercise.ChallengeAttempt{failed, passed, running}, nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(progressRubric(), nil).Times(2)
	f.expectEvaluations(ctx, "attempt-1",
		ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindDiagram, 60, 3)),
		ptr(progressEvaluation("attempt-1", domainexercise.EvaluationKindUnderstanding, 90, 3)))
	f.expectEvaluations(ctx, "attempt-2",
		ptr(progressEvaluation("attempt-2", domainexercise.EvaluationKindDiagram, 92, 3)),
		ptr(progressEvaluation("attempt-2", domainexercise.EvaluationKindUnderstanding, 88, 3)))

	// When reading the challenge's progress
	progress, err := f.service.GetChallengeProgress(ctx, "challenge-1")

	// Then it is completed, with the count, best score and first pass of the stored attempts
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ChallengeProgress{
		Status: domainexercise.ProgressCompleted, Attempts: 3, BestDiagramScore: 92, FirstPassedAt: passed.SubmittedAt,
	}, progress)
}

func TestProgressService_GetChallengeProgress_isNotStartedWithoutAttempts(t *testing.T) {
	f := newProgressFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, nil)

	progress, err := f.service.GetChallengeProgress(ctx, "challenge-1")

	require.NoError(t, err)
	assert.Equal(t, domainexercise.ProgressNotStarted, progress.Status)
}

func TestProgressService_GetChallengeProgress_reportsStoreFailures(t *testing.T) {
	failure := errors.New("disk failure")
	ctx := context.Background()

	f := newProgressFixture(t)
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, failure)
	_, err := f.service.GetChallengeProgress(ctx, "challenge-1")
	assert.ErrorIs(t, err, failure)

	f = newProgressFixture(t)
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").
		Return([]domainexercise.ChallengeAttempt{progressAttempt("attempt-1", domainexercise.AttemptEvaluated, 80)}, nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(domainexercise.Evaluation{}, failure)
	_, err = f.service.GetChallengeProgress(ctx, "challenge-1")
	assert.ErrorIs(t, err, failure)
}
