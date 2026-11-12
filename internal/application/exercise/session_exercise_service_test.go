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

type sessionExerciseFixture struct {
	service    *SessionExerciseService
	links      *domainmocks.MockSessionExerciseRepository
	challenges *domainmocks.MockChallengeRepository
	records    *domainmocks.MockAttemptRecordRepository
}

func newSessionExerciseFixture(t *testing.T) *sessionExerciseFixture {
	t.Helper()
	f := &sessionExerciseFixture{
		links:      domainmocks.NewMockSessionExerciseRepository(t),
		challenges: domainmocks.NewMockChallengeRepository(t),
		records:    domainmocks.NewMockAttemptRecordRepository(t),
	}
	f.service = NewSessionExerciseService(f.links, f.challenges, f.records)
	return f
}

func (f *sessionExerciseFixture) expectLinks(links ...domainexercise.SessionExercise) {
	f.links.EXPECT().ListBySession(context.Background(), "session-1").Return(links, nil).Once()
}

func cardLink(id, challengeID, messageID string) domainexercise.SessionExercise {
	return domainexercise.SessionExercise{ID: id, SessionID: "session-1", ChallengeID: challengeID, MessageID: messageID}
}

func cardChallenge(id, title string) domainexercise.Challenge {
	return domainexercise.Challenge{
		ID: id, Title: title, Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced,
		TimeLimitMinutes: 60, Scenario: "secret scenario",
	}
}

// finishedRecord is a finished attempt submitted on the given day whose two
// evaluations pass (level 4) or fail (level 1) the mandatory criterion under a
// minimum of 70.
func finishedRecord(id string, day, level, diagramScore, understandingScore int) domainexercise.AttemptRecord {
	rubric := domainexercise.RubricVersion{ID: "r", Criteria: []domainexercise.RubricCriterion{{ID: "c1", Weight: 100, Mandatory: true}}}
	results := []domainexercise.CriterionResult{{CriterionID: "c1", Level: level}}
	return domainexercise.AttemptRecord{
		Attempt: domainexercise.ChallengeAttempt{
			ID: id, State: domainexercise.AttemptFinished, PassScore: 70,
			SubmittedAt: time.Date(2026, 11, day, 10, 0, 0, 0, time.UTC),
		},
		Diagram:       &domainexercise.Evaluation{Kind: domainexercise.EvaluationKindDiagram, Score: diagramScore, Results: results},
		Understanding: &domainexercise.Evaluation{Kind: domainexercise.EvaluationKindUnderstanding, Score: understandingScore, Results: results},
		Rubric:        &rubric,
	}
}

func TestSessionExerciseService_ListSessionExercises_aChallengeNeverTriedIsNotStartedWithoutLatest(t *testing.T) {
	// Given a session linked to a challenge with no attempts
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "Design a feed"), nil).Once()
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).
		Return(map[string][]domainexercise.AttemptRecord{}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then the card describes the challenge without its scenario and has no latest result
	require.NoError(t, err)
	assert.Equal(t, []SessionExerciseCard{{
		SessionExerciseID: "link-1", MessageID: "message-1", ChallengeID: "challenge-1", Title: "Design a feed",
		Difficulty: domainexercise.DifficultyAdvanced, TimeLimitMinutes: 60,
		Status: domainexercise.ProgressNotStarted, Attempts: 0, Latest: nil,
	}}, cards)
}

func TestSessionExerciseService_ListSessionExercises_reportsTheLatestFinishedAttemptAndCountsAllFinishedOnes(t *testing.T) {
	// Given a challenge passed on day 3 and failed on day 5, plus a running attempt and one from another session
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "Design a feed"), nil).Once()
	other := finishedRecord("other", 1, 1, 10, 20)
	other.Attempt.SessionExerciseID = "another-link"
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).Return(map[string][]domainexercise.AttemptRecord{
		"challenge-1": {
			other,
			finishedRecord("passed", 3, 4, 88, 91),
			finishedRecord("failed", 5, 1, 40, 55),
			{Attempt: domainexercise.ChallengeAttempt{ID: "running", State: domainexercise.AttemptInProgress}},
		},
	}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then only finished attempts are counted, a past pass keeps it completed and the latest is the day-5 failure
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.Equal(t, domainexercise.ProgressCompleted, cards[0].Status)
	assert.Equal(t, 3, cards[0].Attempts)
	assert.Equal(t, &LatestResult{Outcome: domainexercise.ResultFailed, DiagramScore: 40, UnderstandingScore: 55}, cards[0].Latest)
}

func TestSessionExerciseService_ListSessionExercises_aRunningAttemptMakesTheStatusInProgressLikeTheCatalog(t *testing.T) {
	// Given a challenge with one failed finished attempt and one running attempt
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "Design a feed"), nil).Once()
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).Return(map[string][]domainexercise.AttemptRecord{
		"challenge-1": {
			finishedRecord("failed", 5, 1, 40, 55),
			{Attempt: domainexercise.ChallengeAttempt{ID: "running", State: domainexercise.AttemptInProgress}},
		},
	}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then the status is In progress, while the count and the latest stay with the finished attempt
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ProgressInProgress, cards[0].Status)
	assert.Equal(t, 1, cards[0].Attempts)
	assert.Equal(t, domainexercise.ResultFailed, cards[0].Latest.Outcome)
}

func TestSessionExerciseService_ListSessionExercises_anAttemptStillBeingEvaluatedMakesTheStatusInProgress(t *testing.T) {
	// Given a challenge whose only attempt was submitted and is not finished
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "Design a feed"), nil).Once()
	submitted := domainexercise.AttemptRecord{Attempt: domainexercise.ChallengeAttempt{ID: "submitted", State: domainexercise.AttemptSubmitted}}
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).
		Return(map[string][]domainexercise.AttemptRecord{"challenge-1": {submitted}}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then the catalog would say In progress, so the card does, with no finished attempt to report
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ProgressInProgress, cards[0].Status)
	assert.Equal(t, 0, cards[0].Attempts)
	assert.Nil(t, cards[0].Latest)
}

func TestSessionExerciseService_ListSessionExercises_aFailedOnlyChallengeNeedsImprovement(t *testing.T) {
	// Given a challenge with one failed finished attempt
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "Design a feed"), nil).Once()
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).Return(map[string][]domainexercise.AttemptRecord{
		"challenge-1": {finishedRecord("failed", 5, 1, 40, 55)},
	}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then it needs improvement
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ProgressNeedsImprovement, cards[0].Status)
	assert.Equal(t, 1, cards[0].Attempts)
}

func TestSessionExerciseService_ListSessionExercises_ignoresAFinishedAttemptWithoutEvaluations(t *testing.T) {
	// Given a finished attempt that has no evaluations
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "T"), nil).Once()
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).Return(map[string][]domainexercise.AttemptRecord{
		"challenge-1": {recordWithoutEvaluations()},
	}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then it is not a result
	require.NoError(t, err)
	assert.Nil(t, cards[0].Latest)
	assert.Equal(t, 0, cards[0].Attempts)
}

func recordWithoutEvaluations() domainexercise.AttemptRecord {
	return domainexercise.AttemptRecord{Attempt: domainexercise.ChallengeAttempt{ID: "bare", State: domainexercise.AttemptFinished}}
}

func TestSessionExerciseService_ListSessionExercises_aSessionWithoutLinksYieldsAnEmptyListWithoutReadingAttempts(t *testing.T) {
	// Given a session with no links
	f := newSessionExerciseFixture(t)
	f.expectLinks()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then the list is empty and not nil
	require.NoError(t, err)
	assert.Equal(t, []SessionExerciseCard{}, cards)
}

func TestSessionExerciseService_ListSessionExercises_keepsTheLinkOrderWithOneCardEach(t *testing.T) {
	// Given two links to two challenges
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"), cardLink("link-2", "challenge-2", "message-2"))
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "First"), nil).Once()
	f.challenges.EXPECT().Get(context.Background(), "challenge-2").Return(cardChallenge("challenge-2", "Second"), nil).Once()
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1", "challenge-2"}).
		Return(map[string][]domainexercise.AttemptRecord{"challenge-2": {finishedRecord("a", 5, 4, 90, 90)}}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then the cards follow the link order and each has its own progress
	require.NoError(t, err)
	require.Len(t, cards, 2)
	assert.Equal(t, "First", cards[0].Title)
	assert.Equal(t, domainexercise.ProgressNotStarted, cards[0].Status)
	assert.Equal(t, "Second", cards[1].Title)
	assert.Equal(t, domainexercise.ProgressCompleted, cards[1].Status)
}

func TestSessionExerciseService_ListSessionExercises_wrapsRepositoryFailures(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		setup func(f *sessionExerciseFixture)
	}{
		{"links", func(f *sessionExerciseFixture) {
			f.links.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, boom).Once()
		}},
		{"challenge", func(f *sessionExerciseFixture) {
			f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
			f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(domainexercise.Challenge{}, boom).Once()
		}},
		{"attempts", func(f *sessionExerciseFixture) {
			f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
			f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "T"), nil).Once()
			f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).Return(nil, boom).Once()
		}},
		{"invalid attempt", func(f *sessionExerciseFixture) {
			f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
			f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(cardChallenge("challenge-1", "T"), nil).Once()
			broken := finishedRecord("broken", 5, 4, 90, 90)
			broken.Attempt.PassScore = 0
			f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).
				Return(map[string][]domainexercise.AttemptRecord{"challenge-1": {broken}}, nil).Once()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing dependency
			f := newSessionExerciseFixture(t)
			tt.setup(f)

			// When listing the session's exercises
			_, err := f.service.ListSessionExercises(context.Background(), "session-1")

			// Then the error is returned
			require.Error(t, err)
		})
	}
}

func TestSessionExerciseService_ListSessionExercises_carriesTheKindAndTheUntimedLimit(t *testing.T) {
	// Given a link to a practice challenge with no time limit
	f := newSessionExerciseFixture(t)
	f.expectLinks(cardLink("link-1", "challenge-1", "message-1"))
	practice := cardChallenge("challenge-1", "Cache aside")
	practice.Kind = domainexercise.ChallengeKindPractice
	practice.TimeLimitMinutes = 0
	f.challenges.EXPECT().Get(context.Background(), "challenge-1").Return(practice, nil).Once()
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"challenge-1"}).
		Return(map[string][]domainexercise.AttemptRecord{}, nil).Once()

	// When listing the session's exercises
	cards, err := f.service.ListSessionExercises(context.Background(), "session-1")

	// Then the card says it is a practice with no time limit
	require.NoError(t, err)
	require.Len(t, cards, 1)
	assert.Equal(t, domainexercise.ChallengeKindPractice, cards[0].Kind)
	assert.Equal(t, 0, cards[0].TimeLimitMinutes)
}
