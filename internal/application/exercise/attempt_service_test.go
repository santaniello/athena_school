package exercise

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
