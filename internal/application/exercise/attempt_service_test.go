package exercise

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
func (f *attemptFixture) runTx(ctx context.Context) {
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

func attemptChallenge() domainexercise.Challenge {
	return domainexercise.Challenge{
		ID: "challenge-1", Title: "Design a URL shortener", Topic: "caching", TimeLimitMinutes: 45,
		Scenario:                  "A link service.",
		FunctionalRequirements:    []string{"Shorten a URL"},
		NonFunctionalRequirements: []string{"p99 under 100 ms"},
		Constraints:               []string{"No new database"},
		Objectives:                []string{"Cut read load"},
	}
}

func attemptStatement() ChallengeStatement {
	return ChallengeStatement{
		Scenario:                  "A link service.",
		FunctionalRequirements:    []string{"Shorten a URL"},
		NonFunctionalRequirements: []string{"p99 under 100 ms"},
		Constraints:               []string{"No new database"},
		Objectives:                []string{"Cut read load"},
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
	f.runTx(ctx)
	f.attempts.EXPECT().Create(ctx, expected).Return(nil)

	// When starting a standalone attempt
	view, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then the attempt is stored and returned with the challenge and rubric details
	require.NoError(t, err)
	assert.Equal(t, AttemptView{
		Attempt:        expected,
		ChallengeTitle: "Design a URL shortener",
		ChallengeTopic: "caching",
		Statement:      attemptStatement(),
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
	f.runTx(ctx)
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
	f.runTx(ctx)
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
	f.runTx(ctx)
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
	f.runTx(ctx)
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
	f.runTx(ctx)
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
	f.runTx(ctx)
	f.attempts.EXPECT().Create(ctx, startedAttempt()).Return(domainexercise.ErrAnotherAttemptInProgress)

	// When starting
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1"})

	// Then the conflict is still reported, without an id
	var conflict *AttemptInProgressError
	require.ErrorAs(t, err, &conflict)
	assert.Empty(t, conflict.AttemptID)
}

func TestAttemptService_StartAttempt_sessionStartCarriesTheAcceptedLinkAndCreatesNone(t *testing.T) {
	// Given a session linked to the challenge when the user accepted the offer
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.sessions.EXPECT().Exists(ctx, "session-1").Return(true, nil)
	f.links.EXPECT().GetBySessionAndChallenge(ctx, "session-1", "challenge-1").
		Return(domainexercise.SessionExercise{ID: "link-1", SessionID: "session-1", ChallengeID: "challenge-1"}, nil)
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)
	f.runTx(ctx)
	linked := startedAttempt()
	linked.SessionExerciseID = "link-1"
	f.attempts.EXPECT().Create(ctx, linked).Return(nil)

	// When starting from the session
	view, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", SessionID: "session-1"})

	// Then the attempt carries the existing link (SessionExerciseRepository.Create is never called)
	require.NoError(t, err)
	assert.Equal(t, "link-1", view.Attempt.SessionExerciseID)
}

func TestAttemptService_StartAttempt_unknownSessionStoresNothing(t *testing.T) {
	// Given a session id that does not exist
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.sessions.EXPECT().Exists(ctx, "ghost").Return(false, nil)

	// When starting from it
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", SessionID: "ghost"})

	// Then it fails with ErrSourceSessionNotFound
	assert.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestAttemptService_StartAttempt_sessionNotLinkedToTheChallengeStoresNothing(t *testing.T) {
	// Given a session that exists but was never offered this challenge
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.expectStartable(ctx)
	f.sessions.EXPECT().Exists(ctx, "session-1").Return(true, nil)
	f.links.EXPECT().GetBySessionAndChallenge(ctx, "session-1", "challenge-1").
		Return(domainexercise.SessionExercise{}, domainexercise.ErrSessionExerciseNotFound)

	// When starting from it
	_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", SessionID: "session-1"})

	// Then it fails with ErrSessionExerciseNotFound
	assert.ErrorIs(t, err, domainexercise.ErrSessionExerciseNotFound)
}

func TestAttemptService_StartAttempt_sessionLookupFailuresAreReturned(t *testing.T) {
	boom := errors.New("disk failure")
	tests := []struct {
		name  string
		setup func(f *attemptFixture, ctx context.Context)
	}{
		{"session check fails", func(f *attemptFixture, ctx context.Context) {
			f.sessions.EXPECT().Exists(ctx, "session-1").Return(false, boom)
		}},
		{"link lookup fails", func(f *attemptFixture, ctx context.Context) {
			f.sessions.EXPECT().Exists(ctx, "session-1").Return(true, nil)
			f.links.EXPECT().GetBySessionAndChallenge(ctx, "session-1", "challenge-1").
				Return(domainexercise.SessionExercise{}, boom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing lookup
			f := newAttemptFixture(t)
			ctx := context.Background()
			f.expectStartable(ctx)
			tt.setup(f, ctx)

			// When starting from the session
			_, err := f.service.StartAttempt(ctx, StartInput{ChallengeID: "challenge-1", SessionID: "session-1"})

			// Then the failure surfaces as itself
			require.ErrorIs(t, err, boom)
			assert.NotErrorIs(t, err, ErrSourceSessionNotFound)
			assert.NotErrorIs(t, err, domainexercise.ErrSessionExerciseNotFound)
		})
	}
}

// expectView arms the lookups that build an AttemptView for attemptChallenge.
func (f *attemptFixture) expectView(ctx context.Context) {
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(attemptRubric(), nil)
}

// storedAttempt is an IN_PROGRESS attempt whose deadline is fixedNow plus 45 minutes.
func storedAttempt() domainexercise.ChallengeAttempt {
	attempt := startedAttempt()
	attempt.ID = "attempt-1"
	return attempt
}

// expectedSubmission is the pair SubmitAttempt stores for storedAttempt
// submitted with "<mxfile/>" at now.
func expectedSubmission(now time.Time, reason domainexercise.SubmitReason) (domainexercise.ChallengeAttempt, domainexercise.Submission) {
	submitted := storedAttempt()
	submitted.State = domainexercise.AttemptSubmitted
	submitted.SubmittedAt = now
	submitted.SubmitReason = reason
	sum := sha256.Sum256([]byte("<mxfile/>"))
	return submitted, domainexercise.Submission{
		ID: "submission-1", AttemptID: "attempt-1", ContentType: "application/drawio", Content: "<mxfile/>",
		Checksum: hex.EncodeToString(sum[:]), SubmittedAt: now,
	}
}

func TestAttemptService_SubmitAttempt_acceptsAtTheDeadlineBoundaries(t *testing.T) {
	deadline := storedAttempt().Deadline
	tests := []struct {
		name       string
		now        time.Time
		wantReason domainexercise.SubmitReason
	}{
		{"just before the deadline is a user submit", deadline.Add(-time.Nanosecond), domainexercise.SubmitReasonUser},
		{"exactly at the deadline is time expired", deadline, domainexercise.SubmitReasonTimeExpired},
		{"exactly at the end of the grace period is time expired", deadline.Add(15 * time.Second), domainexercise.SubmitReasonTimeExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an attempt in progress and a clock at the boundary
			f := newAttemptFixture(t)
			ctx := context.Background()
			f.service.newID = func() string { return "submission-1" }
			f.clock.now = tt.now
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
			submitted, submission := expectedSubmission(tt.now, tt.wantReason)
			f.attempts.EXPECT().Submit(ctx, submitted, submission).Return(nil)
			f.expectView(ctx)

			// When submitting
			view, err := f.service.SubmitAttempt(ctx, "attempt-1", "application/drawio", "<mxfile/>")

			// Then the attempt is stored as submitted for the expected reason and returned with its submission
			require.NoError(t, err)
			assert.Equal(t, submitted, view.Attempt)
			require.NotNil(t, view.Submission)
			assert.Equal(t, "submission-1", view.Submission.ID)
			assert.Equal(t, "Design a URL shortener", view.ChallengeTitle)
		})
	}
}

func TestAttemptService_SubmitAttempt_tooLateDiscardsTheAttempt(t *testing.T) {
	// Given an attempt whose grace period ended a nanosecond ago
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.clock.now = storedAttempt().Deadline.Add(15*time.Second + time.Nanosecond)
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
	f.attempts.EXPECT().DeleteUnfinished(ctx, "attempt-1").Return(nil)

	// When submitting
	_, err := f.service.SubmitAttempt(ctx, "attempt-1", "application/drawio", "<mxfile/>")

	// Then it is refused as too late and the attempt is gone (Submit is never called)
	assert.ErrorIs(t, err, ErrSubmissionTooLate)
}

func TestAttemptService_SubmitAttempt_tooLateWithAFailedDiscardReturnsOnlyTheFailure(t *testing.T) {
	// Given a late submission whose attempt cannot be deleted
	f := newAttemptFixture(t)
	ctx := context.Background()
	boom := errors.New("disk failure")
	f.clock.now = storedAttempt().Deadline.Add(time.Hour)
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
	f.attempts.EXPECT().DeleteUnfinished(ctx, "attempt-1").Return(boom)

	// When submitting
	_, err := f.service.SubmitAttempt(ctx, "attempt-1", "application/drawio", "<mxfile/>")

	// Then only the failure is returned, since the attempt may still be stored
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrSubmissionTooLate)
}

func TestAttemptService_SubmitAttempt_secondSubmitIsRefusedEvenAfterTheDeadline(t *testing.T) {
	// Given an attempt already submitted, long past its deadline
	f := newAttemptFixture(t)
	ctx := context.Background()
	already := storedAttempt()
	already.State = domainexercise.AttemptSubmitted
	f.clock.now = already.Deadline.Add(time.Hour)
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(already, nil)

	// When submitting again
	_, err := f.service.SubmitAttempt(ctx, "attempt-1", "application/drawio", "<mxfile/>")

	// Then it fails with ErrAttemptNotInProgress and nothing is stored or deleted
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotInProgress)
}

func TestAttemptService_SubmitAttempt_unknownAttempt(t *testing.T) {
	// Given no such attempt
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "ghost").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When submitting
	_, err := f.service.SubmitAttempt(ctx, "ghost", "application/drawio", "<mxfile/>")

	// Then it fails with ErrAttemptNotFound
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
}

func TestAttemptService_SubmitAttempt_invalidContentStoresNothing(t *testing.T) {
	// Given an attempt in progress
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)

	// When submitting empty content
	_, err := f.service.SubmitAttempt(ctx, "attempt-1", "application/drawio", "")

	// Then the domain's error is returned and the attempt stays in progress
	assert.ErrorIs(t, err, domainexercise.ErrSubmissionContentRequired)
}

func TestAttemptService_SubmitAttempt_storageAndReadFailuresAreReturned(t *testing.T) {
	boom := errors.New("disk failure")
	tests := []struct {
		name  string
		setup func(f *attemptFixture, ctx context.Context)
	}{
		{"submit fails", func(f *attemptFixture, ctx context.Context) {
			submitted, submission := expectedSubmission(fixedNow, domainexercise.SubmitReasonUser)
			f.attempts.EXPECT().Submit(ctx, submitted, submission).Return(boom)
		}},
		{"challenge read fails", func(f *attemptFixture, ctx context.Context) {
			submitted, submission := expectedSubmission(fixedNow, domainexercise.SubmitReasonUser)
			f.attempts.EXPECT().Submit(ctx, submitted, submission).Return(nil)
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, boom)
		}},
		{"rubric read fails", func(f *attemptFixture, ctx context.Context) {
			submitted, submission := expectedSubmission(fixedNow, domainexercise.SubmitReasonUser)
			f.attempts.EXPECT().Submit(ctx, submitted, submission).Return(nil)
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
			f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(domainexercise.RubricVersion{}, boom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing dependency
			f := newAttemptFixture(t)
			ctx := context.Background()
			f.service.newID = func() string { return "submission-1" }
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
			tt.setup(f, ctx)

			// When submitting
			_, err := f.service.SubmitAttempt(ctx, "attempt-1", "application/drawio", "<mxfile/>")

			// Then the failure is returned
			assert.ErrorIs(t, err, boom)
		})
	}
}

func TestAttemptService_DiscardAttempt_deletesAnUnfinishedAttemptInAnyState(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{
		domainexercise.AttemptInProgress, domainexercise.AttemptSubmitted, domainexercise.AttemptEvaluating,
		domainexercise.AttemptEvaluationFailed, domainexercise.AttemptEvaluated,
	} {
		t.Run(string(state), func(t *testing.T) {
			// Given an unfinished attempt
			f := newAttemptFixture(t)
			ctx := context.Background()
			attempt := storedAttempt()
			attempt.State = state
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attempt, nil)
			f.attempts.EXPECT().DeleteUnfinished(ctx, "attempt-1").Return(nil)

			// When discarding it
			err := f.service.DiscardAttempt(ctx, "attempt-1")

			// Then it is deleted
			assert.NoError(t, err)
		})
	}
}

func TestAttemptService_DiscardAttempt_refusesAFinishedAttempt(t *testing.T) {
	// Given a finished attempt
	f := newAttemptFixture(t)
	ctx := context.Background()
	attempt := storedAttempt()
	attempt.State = domainexercise.AttemptFinished
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attempt, nil)

	// When discarding it
	err := f.service.DiscardAttempt(ctx, "attempt-1")

	// Then it is refused and nothing is deleted
	assert.ErrorIs(t, err, ErrAttemptFinished)
}

func TestAttemptService_DiscardAttempt_failuresAreReturned(t *testing.T) {
	boom := errors.New("disk failure")
	tests := []struct {
		name  string
		setup func(f *attemptFixture, ctx context.Context)
		want  error
	}{
		{"unknown attempt", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)
		}, domainexercise.ErrAttemptNotFound},
		{"delete fails", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
			f.attempts.EXPECT().DeleteUnfinished(ctx, "attempt-1").Return(boom)
		}, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing dependency
			f := newAttemptFixture(t)
			ctx := context.Background()
			tt.setup(f, ctx)

			// When discarding
			err := f.service.DiscardAttempt(ctx, "attempt-1")

			// Then the failure is returned
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestAttemptService_DiscardStaleAttempts_returnsHowManyWereRemoved(t *testing.T) {
	// Given three unfinished attempts left behind
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().DeleteAllUnfinished(ctx).Return(3, nil)

	// When cleaning up
	removed, err := f.service.DiscardStaleAttempts(ctx)

	// Then the count is reported
	require.NoError(t, err)
	assert.Equal(t, 3, removed)
}

func TestAttemptService_DiscardStaleAttempts_returnsTheFailure(t *testing.T) {
	// Given a failing cleanup
	f := newAttemptFixture(t)
	ctx := context.Background()
	boom := errors.New("disk failure")
	f.attempts.EXPECT().DeleteAllUnfinished(ctx).Return(0, boom)

	// When cleaning up
	_, err := f.service.DiscardStaleAttempts(ctx)

	// Then the caller gets the error to log
	assert.ErrorIs(t, err, boom)
}

func submittedAttempt() domainexercise.ChallengeAttempt {
	attempt := storedAttempt()
	attempt.State = domainexercise.AttemptSubmitted
	attempt.SubmittedAt = fixedNow
	attempt.SubmitReason = domainexercise.SubmitReasonUser
	return attempt
}

func TestAttemptService_GetAttempt_inProgressViewHasNoSubmission(t *testing.T) {
	// Given an attempt in progress
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
	f.expectView(ctx)

	// When reading it
	view, err := f.service.GetAttempt(ctx, "attempt-1")

	// Then the view has the challenge and criteria but no submission (and none is looked up)
	require.NoError(t, err)
	assert.Equal(t, AttemptView{
		Attempt: storedAttempt(), ChallengeTitle: "Design a URL shortener", ChallengeTopic: "caching",
		Statement: attemptStatement(), Criteria: attemptRubric().Criteria,
	}, view)
}

func TestAttemptService_GetAttempt_submittedViewCarriesTheSubmission(t *testing.T) {
	// Given a submitted attempt
	f := newAttemptFixture(t)
	ctx := context.Background()
	stored := domainexercise.Submission{ID: "submission-1", AttemptID: "attempt-1", Content: "<mxfile/>"}
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(submittedAttempt(), nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(stored, nil)
	f.expectView(ctx)

	// When reading it
	view, err := f.service.GetAttempt(ctx, "attempt-1")

	// Then the view includes the submission
	require.NoError(t, err)
	require.NotNil(t, view.Submission)
	assert.Equal(t, stored, *view.Submission)
}

func TestAttemptService_GetAttempt_failuresAreReturned(t *testing.T) {
	boom := errors.New("disk failure")
	tests := []struct {
		name  string
		setup func(f *attemptFixture, ctx context.Context)
		want  error
	}{
		{"unknown attempt", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)
		}, domainexercise.ErrAttemptNotFound},
		{"submitted attempt without a submission", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(submittedAttempt(), nil)
			f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(domainexercise.Submission{}, domainexercise.ErrSubmissionNotFound)
		}, domainexercise.ErrSubmissionNotFound},
		{"challenge read fails", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedAttempt(), nil)
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, boom)
		}, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing dependency
			f := newAttemptFixture(t)
			ctx := context.Background()
			tt.setup(f, ctx)

			// When reading the attempt
			_, err := f.service.GetAttempt(ctx, "attempt-1")

			// Then the failure is returned
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestAttemptService_ListAttempts_returnsAViewPerAttemptOldestFirst(t *testing.T) {
	// Given a challenge with a submitted attempt and a running one
	f := newAttemptFixture(t)
	ctx := context.Background()
	second := storedAttempt()
	second.ID = "attempt-2"
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return([]domainexercise.ChallengeAttempt{submittedAttempt(), second}, nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(domainexercise.Submission{ID: "submission-1"}, nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil).Twice()
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(attemptRubric(), nil).Twice()

	// When listing
	views, err := f.service.ListAttempts(ctx, "challenge-1")

	// Then there is one view per attempt, in order
	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, "attempt-1", views[0].Attempt.ID)
	assert.NotNil(t, views[0].Submission)
	assert.Equal(t, "attempt-2", views[1].Attempt.ID)
	assert.Nil(t, views[1].Submission)
}

func TestAttemptService_ListAttempts_noAttemptsIsAnEmptyList(t *testing.T) {
	// Given a challenge nobody attempted
	f := newAttemptFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, nil)

	// When listing
	views, err := f.service.ListAttempts(ctx, "challenge-1")

	// Then the list is empty, not nil, so the UI gets []
	require.NoError(t, err)
	assert.NotNil(t, views)
	assert.Empty(t, views)
}

func TestAttemptService_ListAttempts_failuresAreReturned(t *testing.T) {
	boom := errors.New("disk failure")
	tests := []struct {
		name  string
		setup func(f *attemptFixture, ctx context.Context)
	}{
		{"list fails", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return(nil, boom)
		}},
		{"a view fails", func(f *attemptFixture, ctx context.Context) {
			f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return([]domainexercise.ChallengeAttempt{storedAttempt()}, nil)
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, boom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing dependency
			f := newAttemptFixture(t)
			ctx := context.Background()
			tt.setup(f, ctx)

			// When listing
			_, err := f.service.ListAttempts(ctx, "challenge-1")

			// Then the failure is returned
			assert.ErrorIs(t, err, boom)
		})
	}
}
