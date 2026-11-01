package exercise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

// fakeClock is a Clock the test moves by hand, so no test sleeps.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

type attemptFixture struct {
	service     *AttemptService
	challenges  *domainmocks.MockChallengeRepository
	rubrics     *domainmocks.MockRubricRepository
	attempts    *domainmocks.MockAttemptRepository
	submissions *domainmocks.MockSubmissionRepository
	links       *domainmocks.MockSessionExerciseRepository
	sessions    *exercisemocks.MockSessionExists
	tx          *exercisemocks.MockTransactor
	clock       *fakeClock
}

func newAttemptFixture(t *testing.T) *attemptFixture {
	t.Helper()
	f := &attemptFixture{
		challenges:  domainmocks.NewMockChallengeRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		attempts:    domainmocks.NewMockAttemptRepository(t),
		submissions: domainmocks.NewMockSubmissionRepository(t),
		links:       domainmocks.NewMockSessionExerciseRepository(t),
		sessions:    exercisemocks.NewMockSessionExists(t),
		tx:          exercisemocks.NewMockTransactor(t),
		clock:       &fakeClock{now: fixedNow},
	}
	f.service = NewAttemptService(f.challenges, f.rubrics, f.attempts, f.submissions, f.links, f.sessions, f.tx, f.clock)
	f.service.newID = func() string { return "attempt-1" }
	return f
}

// runTx makes the mocked Transactor run its function, so the repository calls
// made inside it are observed.
func (f *attemptFixture) runTx() {
	f.tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

func attemptChallenge() domainexercise.Challenge {
	return domainexercise.Challenge{
		ID: "challenge-1", Title: "Design a URL shortener", Topic: "caching", TimeLimitMinutes: 45,
	}
}

func attemptRubric() domainexercise.RubricVersion {
	return domainexercise.RubricVersion{
		ID: "rubric-2", ChallengeID: "challenge-1", Version: 2,
		Criteria: []domainexercise.RubricCriterion{{ID: "c1", Key: "architecture", Weight: 100}},
	}
}

func TestAttemptService_StartAttempt_standaloneFreezesLatestRubricAndSetsDeadline(t *testing.T) {
	// Given a challenge with a rubric, and no attempt running
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Latest(ctx, "challenge-1").Return(attemptRubric(), nil)
	expected := domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-2",
		State: domainexercise.AttemptInProgress, StartedAt: fixedNow, Deadline: fixedNow.Add(45 * time.Minute),
	}
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)
	f.runTx()
	f.attempts.EXPECT().Create(ctx, expected).Return(nil)

	// When starting a standalone attempt
	view, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then the attempt is stored and returned with the challenge and rubric details
	require.NoError(t, err)
	assert.Equal(t, AttemptView{
		Attempt:        expected,
		ChallengeTitle: "Design a URL shortener",
		ChallengeTopic: "caching",
		Criteria:       attemptRubric().Criteria,
	}, view)
}

func TestAttemptService_StartAttempt_challengeWithoutRubricStoresNothing(t *testing.T) {
	// Given a challenge that has no rubric
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)

	// When starting an attempt on it
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then it fails with ErrChallengeHasNoRubric (and Create is never called)
	assert.ErrorIs(t, err, ErrChallengeHasNoRubric)
}

func TestAttemptService_StartAttempt_unknownChallengeStoresNothing(t *testing.T) {
	// Given no such challenge
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().Get(ctx, "missing").Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)

	// When starting an attempt on it
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "missing"})

	// Then it fails with ErrChallengeNotFound
	assert.ErrorIs(t, err, domainexercise.ErrChallengeNotFound)
}

func TestAttemptService_StartAttempt_returnsStorageFailures(t *testing.T) {
	// Given a rubric lookup that fails for another reason than "not found"
	f := newAttemptFixture(t)
	ctx := context.Background()
	boom := errors.New("disk failure")
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{}, boom)

	// When starting an attempt
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then the failure surfaces and is not mistaken for a missing rubric
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrChallengeHasNoRubric)
}

// startedAttempt is what StartAttempt stores for attemptChallenge at fixedNow.
func startedAttempt() domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-2",
		State: domainexercise.AttemptInProgress, StartedAt: fixedNow, Deadline: fixedNow.Add(45 * time.Minute),
	}
}

func runningAttempt() domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{ID: "running-1", ChallengeID: "challenge-0", State: domainexercise.AttemptInProgress}
}

// expectStartable arms the challenge and rubric lookups every start performs.
func (f *attemptFixture) expectStartable(ctx context.Context) {
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Latest(ctx, "challenge-1").Return(attemptRubric(), nil)
}

func TestAttemptService_StartAttempt_refusesWhileAnotherAttemptRunsAndLeavesItUntouched(t *testing.T) {
	// Given an attempt already running
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(runningAttempt(), nil)

	// When starting another without confirming the discard
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then it fails with ErrAttemptInProgress naming the running attempt, and nothing is deleted or created
	require.ErrorIs(t, err, ErrAttemptInProgress)
	var conflict *AttemptInProgressError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "running-1", conflict.AttemptID)
}

func TestAttemptService_StartAttempt_discardInProgressReplacesTheRunningAttemptInOneTransaction(t *testing.T) {
	// Given an attempt already running
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(runningAttempt(), nil)
	f.runTx()
	f.attempts.EXPECT().DeleteUnfinished(ctx, "running-1").Return(nil)
	f.attempts.EXPECT().Create(ctx, domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-2",
		State: domainexercise.AttemptInProgress, StartedAt: fixedNow, Deadline: fixedNow.Add(45 * time.Minute),
	}).Return(nil)

	// When starting another with DiscardInProgress
	view, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", DiscardInProgress: true})

	// Then the old one is deleted and the new one created inside the transaction
	require.NoError(t, err)
	assert.Equal(t, "attempt-1", view.Attempt.ID)
}

func TestAttemptService_StartAttempt_discardInProgressWithNothingRunningDeletesNothing(t *testing.T) {
	// Given no attempt running
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)
	f.runTx()
	f.attempts.EXPECT().Create(ctx, startedAttempt()).Return(nil)

	// When starting with DiscardInProgress
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", DiscardInProgress: true})

	// Then it simply starts (DeleteUnfinished is never called)
	require.NoError(t, err)
}

func TestAttemptService_StartAttempt_failedCreateAfterDiscardReturnsTheErrorFromTheTransaction(t *testing.T) {
	// Given a running attempt and a create that fails
	f := newAttemptFixture(t)
	ctx := context.Background()
	boom := errors.New("disk failure")
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(runningAttempt(), nil)
	f.runTx()
	f.attempts.EXPECT().DeleteUnfinished(ctx, "running-1").Return(nil)
	f.attempts.EXPECT().Create(ctx, startedAttempt()).Return(boom)

	// When starting with DiscardInProgress
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", DiscardInProgress: true})

	// Then the error is returned so the transaction rolls the delete back
	assert.ErrorIs(t, err, boom)
}

func TestAttemptService_StartAttempt_failedDeleteStopsBeforeCreating(t *testing.T) {
	// Given a running attempt that cannot be deleted
	f := newAttemptFixture(t)
	ctx := context.Background()
	boom := errors.New("disk failure")
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(runningAttempt(), nil)
	f.runTx()
	f.attempts.EXPECT().DeleteUnfinished(ctx, "running-1").Return(boom)

	// When starting with DiscardInProgress
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", DiscardInProgress: true})

	// Then it fails without creating the new attempt
	assert.ErrorIs(t, err, boom)
}

func TestAttemptService_StartAttempt_lookupFailureIsReturned(t *testing.T) {
	// Given a failing running-attempt lookup
	f := newAttemptFixture(t)
	ctx := context.Background()
	boom := errors.New("disk failure")
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, boom)

	// When starting
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then the failure surfaces and is not mistaken for a conflict
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrAttemptInProgress)
}

func TestAttemptService_StartAttempt_concurrentStartLosingTheRaceIsReportedAsInProgress(t *testing.T) {
	// Given the lookup saw nothing, but the store refuses the create because a concurrent start won
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound).Once()
	f.runTx()
	f.attempts.EXPECT().Create(ctx, startedAttempt()).Return(domainexercise.ErrAnotherAttemptInProgress)
	f.attempts.EXPECT().FindInProgress(ctx).Return(runningAttempt(), nil).Once()

	// When starting
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then it fails with ErrAttemptInProgress naming the winner
	var conflict *AttemptInProgressError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, "running-1", conflict.AttemptID)
}

func TestAttemptService_StartAttempt_conflictWhoseWinnerVanishedStillReportsInProgress(t *testing.T) {
	// Given a refused create and a winner that is already gone by the second lookup
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound).Twice()
	f.runTx()
	f.attempts.EXPECT().Create(ctx, startedAttempt()).Return(domainexercise.ErrAnotherAttemptInProgress)

	// When starting
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then the conflict is still reported, without an id
	var conflict *AttemptInProgressError
	require.ErrorAs(t, err, &conflict)
	assert.Empty(t, conflict.AttemptID)
}
