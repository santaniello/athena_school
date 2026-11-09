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

type historyFixture struct {
	service     *HistoryService
	challenges  *domainmocks.MockChallengeRepository
	rubrics     *domainmocks.MockRubricRepository
	attempts    *domainmocks.MockAttemptRepository
	submissions *domainmocks.MockSubmissionRepository
	evaluations *domainmocks.MockEvaluationRepository
	defense     *domainmocks.MockDefenseRepository
	hints       *domainmocks.MockHintRepository
}

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	f := &historyFixture{
		challenges:  domainmocks.NewMockChallengeRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		attempts:    domainmocks.NewMockAttemptRepository(t),
		submissions: domainmocks.NewMockSubmissionRepository(t),
		evaluations: domainmocks.NewMockEvaluationRepository(t),
		defense:     domainmocks.NewMockDefenseRepository(t),
		hints:       domainmocks.NewMockHintRepository(t),
	}
	f.service = NewHistoryService(f.challenges, f.rubrics, f.attempts, f.submissions, f.evaluations, f.defense, f.hints)
	return f
}

func finishedAttempt(id string, state domainexercise.AttemptState) domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: id, ChallengeID: "challenge-1", RubricVersionID: "rubric-2", State: state, PassScore: 70,
		StartedAt: fixedNow, SubmittedAt: fixedNow.Add(30 * time.Minute), SubmitReason: domainexercise.SubmitReasonTimeExpired,
	}
}

func evaluationOfKind(attemptID string, kind domainexercise.EvaluationKind, score int, finishedAt time.Time, results []domainexercise.CriterionResult) domainexercise.Evaluation {
	return domainexercise.Evaluation{
		ID: string(kind) + "-" + attemptID, AttemptID: attemptID, Kind: kind, Score: score, Summary: "summary " + string(kind),
		Strengths: []string{"s"}, Gaps: []string{"g"}, Recommendations: []string{"r"}, Results: results, CreatedAt: finishedAt,
	}
}

func (f *historyFixture) expectEvaluations(ctx context.Context, attemptID string, diagramScore, understandingScore int, finishedAt time.Time) {
	f.evaluations.EXPECT().GetByAttempt(ctx, attemptID, domainexercise.EvaluationKindDiagram).
		Return(evaluationOfKind(attemptID, domainexercise.EvaluationKindDiagram, diagramScore, finishedAt.Add(-time.Minute), defenseDiagram().Results), nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, attemptID, domainexercise.EvaluationKindUnderstanding).
		Return(evaluationOfKind(attemptID, domainexercise.EvaluationKindUnderstanding, understandingScore, finishedAt, understandingResults()), nil)
}

func understandingResults() []domainexercise.CriterionResult {
	return []domainexercise.CriterionResult{
		{CriterionID: "c1", Level: 3, Justification: "explained", Evidence: "answer 1"},
		{CriterionID: "c2", InsufficientEvidence: true},
		{CriterionID: "c3", Level: 2, Justification: "fair"},
		{CriterionID: "c4", InsufficientEvidence: true},
	}
}

func TestHistoryService_ListHistory_listsOnlyFinishedAttemptsNewestFirstWithScoresAndOutcome(t *testing.T) {
	// Given three attempts of a challenge, the middle one still running
	f := newHistoryFixture(t)
	ctx := context.Background()
	older := fixedNow.Add(time.Hour)
	newer := fixedNow.Add(48 * time.Hour)
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return([]domainexercise.ChallengeAttempt{
		finishedAttempt("old", domainexercise.AttemptFinished),
		finishedAttempt("running", domainexercise.AttemptDefending),
		finishedAttempt("new", domainexercise.AttemptFinished),
	}, nil)
	f.expectEvaluations(ctx, "old", 90, 85, older)
	f.expectEvaluations(ctx, "new", 60, 95, newer)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)

	// When listing the history
	items, err := f.service.ListHistory(ctx, "challenge-1")

	// Then only the finished ones come, newest first, each with its outcome at its frozen pass score
	require.NoError(t, err)
	assert.Equal(t, []AttemptListItem{
		{AttemptID: "new", FinishedAt: newer, DiagramScore: 60, UnderstandingScore: 95, Outcome: domainexercise.ResultFailed},
		{AttemptID: "old", FinishedAt: older, DiagramScore: 90, UnderstandingScore: 85, Outcome: domainexercise.ResultPassed},
	}, items)
}

func TestHistoryService_ListHistory_noAttemptsIsAnEmptyList(t *testing.T) {
	// Given a challenge without attempts
	f := newHistoryFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, nil)

	// When listing the history
	items, err := f.service.ListHistory(ctx, "challenge-1")

	// Then the list is empty and not nil
	require.NoError(t, err)
	assert.NotNil(t, items)
	assert.Empty(t, items)
}

func TestHistoryService_ListHistory_failsWhenTheAttemptsCannotBeListed(t *testing.T) {
	// Given a store that fails
	f := newHistoryFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, boom)

	// When listing the history
	_, err := f.service.ListHistory(ctx, "challenge-1")

	// Then the error is wrapped
	require.ErrorIs(t, err, boom)
}

func TestHistoryService_ListHistory_failsWhenAFinishedAttemptHasNoEvaluation(t *testing.T) {
	// Given a finished attempt whose diagram evaluation is missing
	f := newHistoryFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return([]domainexercise.ChallengeAttempt{finishedAttempt("a", domainexercise.AttemptFinished)}, nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, "a", domainexercise.EvaluationKindDiagram).Return(domainexercise.Evaluation{}, domainexercise.ErrEvaluationNotFound)

	// When listing the history
	_, err := f.service.ListHistory(ctx, "challenge-1")

	// Then the inconsistency is reported, not hidden
	require.ErrorIs(t, err, domainexercise.ErrEvaluationNotFound)
}

func TestHistoryService_GetAttemptResult_joinsEverythingTheResultViewShows(t *testing.T) {
	// Given a finished attempt that used two hints
	f := newHistoryFixture(t)
	ctx := context.Background()
	finishedAt := fixedNow.Add(40 * time.Minute)
	attempt := finishedAttempt("attempt-1", domainexercise.AttemptFinished)
	challenge := defenseChallenge()
	challenge.Difficulty = domainexercise.DifficultyIntermediate
	turns := []domainexercise.DefenseTurn{askedTurn(1, "c1", "Why a queue?", "To absorb bursts")}
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attempt, nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(challenge, nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.expectEvaluations(ctx, "attempt-1", 55, 80, finishedAt)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return(turns, nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.Hint{{ID: "h1"}, {ID: "h2"}}, nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(defenseSubmission, nil)

	// When reading the result
	view, err := f.service.GetAttemptResult(ctx, "attempt-1")

	// Then it carries the statement, the attempt facts, the frozen pass score and the outcome
	require.NoError(t, err)
	assert.Equal(t, ChallengeSnapshot{
		ID: "challenge-1", Title: "Design a URL shortener", Topic: "caching", Difficulty: domainexercise.DifficultyIntermediate,
		Statement: attemptStatement(),
	}, view.Challenge)
	assert.Equal(t, attempt, view.Attempt)
	assert.Equal(t, finishedAt, view.FinishedAt)
	assert.Equal(t, 30*time.Minute, view.TimeSpent)
	assert.Equal(t, 2, view.HintsUsed)
	assert.Equal(t, 70, view.PassScore)
	assert.Equal(t, domainexercise.ResultFailed, view.Outcome.Result)
	assert.Equal(t, defenseRubric().Criteria, view.Criteria)
	assert.Equal(t, turns, view.Defense)
	assert.Equal(t, "<mxfile/>", view.Submission)
}

func TestHistoryService_GetAttemptResult_joinsEachResultWithItsCriterionAndKeepsTheNotJudgedOnes(t *testing.T) {
	// Given a finished attempt whose diagram flagged c1 and whose defense never reached c2 and c4
	f := newHistoryFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(finishedAttempt("attempt-1", domainexercise.AttemptFinished), nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(defenseChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.expectEvaluations(ctx, "attempt-1", 55, 80, fixedNow)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.DefenseTurn{}, nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(defenseSubmission, nil)

	// When reading the result
	view, err := f.service.GetAttemptResult(ctx, "attempt-1")

	// Then every criterion has its row in rubric order, with name, weight and the stored result untouched
	require.NoError(t, err)
	rubric := defenseRubric()
	require.Len(t, view.Diagram.Rows, 4)
	assert.Equal(t, rubric.Criteria[0], view.Diagram.Rows[0].Criterion)
	assert.True(t, view.Diagram.Rows[0].Result.InsufficientEvidence)
	assert.Equal(t, rubric.Criteria[1], view.Diagram.Rows[1].Criterion)
	assert.Equal(t, 3, view.Diagram.Rows[1].Result.Level)
	assert.Equal(t, 55, view.Diagram.Evaluation.Score)
	require.Len(t, view.Understanding.Rows, 4)
	assert.Equal(t, rubric.Criteria[3], view.Understanding.Rows[3].Criterion)
	assert.True(t, view.Understanding.Rows[3].Result.InsufficientEvidence)
	assert.Equal(t, "answer 1", view.Understanding.Rows[0].Result.Evidence)
	assert.Equal(t, 0, view.HintsUsed)
	assert.Empty(t, view.Defense)
}

func TestHistoryService_GetAttemptResult_aPassedAttemptHasNoReasons(t *testing.T) {
	// Given a finished attempt with both scores above the frozen minimum and the mandatory criterion met
	f := newHistoryFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(finishedAttempt("attempt-1", domainexercise.AttemptFinished), nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(defenseChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.expectEvaluations(ctx, "attempt-1", 90, 80, fixedNow)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(defenseSubmission, nil)

	// When reading the result
	view, err := f.service.GetAttemptResult(ctx, "attempt-1")

	// Then the outcome is PASSED, decided by the domain
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ResultPassed, view.Outcome.Result)
	assert.Empty(t, view.Outcome.Reasons)
}

func TestHistoryService_GetAttemptResult_anAttemptThatIsNotFinishedFails(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{
		domainexercise.AttemptInProgress, domainexercise.AttemptSubmitted, domainexercise.AttemptEvaluating,
		domainexercise.AttemptEvaluated, domainexercise.AttemptEvaluationFailed, domainexercise.AttemptDefending,
	} {
		t.Run(string(state), func(t *testing.T) {
			// Given an attempt that is not FINISHED
			f := newHistoryFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(finishedAttempt("attempt-1", state), nil)

			// When reading its result
			_, err := f.service.GetAttemptResult(ctx, "attempt-1")

			// Then it is refused, with nothing else read
			require.ErrorIs(t, err, ErrAttemptNotFinished)
		})
	}
}

func TestHistoryService_GetAttemptResult_anUnknownAttemptFails(t *testing.T) {
	// Given no attempt with the id
	f := newHistoryFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "missing").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When reading its result
	_, err := f.service.GetAttemptResult(ctx, "missing")

	// Then it is ErrAttemptNotFound
	require.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
}

func TestHistoryService_GetAttemptResult_failsWhenTheSubmissionCannotBeRead(t *testing.T) {
	// Given a finished attempt whose submission is missing
	f := newHistoryFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(finishedAttempt("attempt-1", domainexercise.AttemptFinished), nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(defenseChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.expectEvaluations(ctx, "attempt-1", 90, 80, fixedNow)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(domainexercise.Submission{}, domainexercise.ErrSubmissionNotFound)

	// When reading the result
	_, err := f.service.GetAttemptResult(ctx, "attempt-1")

	// Then the failure surfaces
	require.ErrorIs(t, err, domainexercise.ErrSubmissionNotFound)
}
