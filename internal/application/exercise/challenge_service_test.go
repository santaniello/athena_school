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

var fixedNow = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func newTestService(t *testing.T) (*ChallengeService, *domainmocks.MockChallengeRepository, *exercisemocks.MockSessionTopicReader) {
	t.Helper()
	repo := domainmocks.NewMockChallengeRepository(t)
	sessions := exercisemocks.NewMockSessionTopicReader(t)
	service := NewChallengeService(repo, sessions)
	service.now = func() time.Time { return fixedNow }
	service.newID = func() string { return "challenge-1" }
	return service, repo, sessions
}

func validInput() CreateChallengeInput {
	return CreateChallengeInput{
		SourceSessionID:        "session-1",
		Title:                  "Design a URL shortener",
		Scenario:               "A startup needs a service that shortens URLs.",
		FunctionalRequirements: []string{"Create a short link"},
		Objectives:             []string{"Justify the storage choice"},
		Difficulty:             domainexercise.DifficultyBeginner,
		Origin:                 domainexercise.OriginManual,
	}
}

func TestChallengeService_Create_snapshotsNormalizedTopicAndPersists(t *testing.T) {
	// Given a session studying "  Distributed Caching "
	service, repo, sessions := newTestService(t)
	ctx := context.Background()
	sessions.EXPECT().ReadSession(ctx, "session-1").
		Return(domainexercise.SourceSession{Topic: "  Distributed Caching ", Goal: "Ace the interview"}, nil)
	input := validInput()
	input.NonFunctionalRequirements = []string{"  99.9% availability "}
	input.Constraints = []string{"Single region"}
	expected := domainexercise.Challenge{
		ID:                        "challenge-1",
		Type:                      domainexercise.ChallengeTypeArchitecture,
		Title:                     "Design a URL shortener",
		Scenario:                  "A startup needs a service that shortens URLs.",
		FunctionalRequirements:    []string{"Create a short link"},
		NonFunctionalRequirements: []string{"99.9% availability"},
		Constraints:               []string{"Single region"},
		Objectives:                []string{"Justify the storage choice"},
		Difficulty:                domainexercise.DifficultyBeginner,
		Topic:                     "distributed caching",
		SourceSessionID:           "session-1",
		Origin:                    domainexercise.OriginManual,
		TimeLimitMinutes:          30,
		DefenseQuestions:          5,
		CreatedAt:                 fixedNow,
		UpdatedAt:                 fixedNow,
	}
	repo.EXPECT().Create(ctx, expected).Return(nil)

	// When creating the challenge
	got, err := service.Create(ctx, input)

	// Then the topic is the session's normalized topic and the stored challenge is returned
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestChallengeService_Create_trimsTextFields(t *testing.T) {
	// Given input with padded title, scenario and list items
	service, repo, sessions := newTestService(t)
	ctx := context.Background()
	sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{Topic: "caching"}, nil)
	input := validInput()
	input.Title = "  Padded title \n"
	input.Scenario = "\tPadded scenario  "
	input.FunctionalRequirements = []string{" First ", "Second  "}
	input.Objectives = []string{"  Goal"}
	repo.EXPECT().Create(ctx, domainexercise.Challenge{
		ID:                        "challenge-1",
		Type:                      domainexercise.ChallengeTypeArchitecture,
		Title:                     "Padded title",
		Scenario:                  "Padded scenario",
		FunctionalRequirements:    []string{"First", "Second"},
		NonFunctionalRequirements: []string{},
		Constraints:               []string{},
		Objectives:                []string{"Goal"},
		Difficulty:                domainexercise.DifficultyBeginner,
		Topic:                     "caching",
		SourceSessionID:           "session-1",
		Origin:                    domainexercise.OriginManual,
		TimeLimitMinutes:          30,
		DefenseQuestions:          5,
		CreatedAt:                 fixedNow,
		UpdatedAt:                 fixedNow,
	}).Return(nil)

	// When creating the challenge
	_, err := service.Create(ctx, input)

	// Then the stored text is trimmed
	require.NoError(t, err)
}

func TestChallengeService_Create_failsForUnknownSourceSession(t *testing.T) {
	// Given a reader that does not know the session
	service, _, sessions := newTestService(t)
	ctx := context.Background()
	sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{}, ErrSourceSessionNotFound)

	// When creating a challenge from it
	_, err := service.Create(ctx, validInput())

	// Then ErrSourceSessionNotFound is returned and nothing is persisted
	require.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestChallengeService_Create_failsWithoutSourceSessionID(t *testing.T) {
	// Given input with a blank source session id
	service, _, _ := newTestService(t)
	input := validInput()
	input.SourceSessionID = "  "

	// When creating the challenge
	_, err := service.Create(context.Background(), input)

	// Then it fails without consulting the reader or the repository
	require.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestChallengeService_Create_returnsReaderFailure(t *testing.T) {
	// Given a reader that fails for a reason other than not found
	service, _, sessions := newTestService(t)
	ctx := context.Background()
	boom := errors.New("boom")
	sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{}, boom)

	// When creating the challenge
	_, err := service.Create(ctx, validInput())

	// Then the failure is wrapped, not mistaken for a missing session
	require.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestChallengeService_Create_failsValidationWithoutPersisting(t *testing.T) {
	// Given a valid session but input without objectives
	service, _, sessions := newTestService(t)
	ctx := context.Background()
	sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{Topic: "caching"}, nil)
	input := validInput()
	input.Objectives = nil

	// When creating the challenge
	_, err := service.Create(ctx, input)

	// Then the specific validation error is returned and the repository is never called
	require.ErrorIs(t, err, domainexercise.ErrObjectivesRequired)
}

func TestChallengeService_Create_failsWhenSessionTopicIsBlank(t *testing.T) {
	// Given a session whose topic is blank
	service, _, sessions := newTestService(t)
	ctx := context.Background()
	sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{Topic: "   "}, nil)

	// When creating the challenge
	_, err := service.Create(ctx, validInput())

	// Then the topic rule rejects it
	require.ErrorIs(t, err, domainexercise.ErrChallengeTopicRequired)
}

func TestChallengeService_Create_returnsRepositoryFailure(t *testing.T) {
	// Given a repository that fails to persist
	service, repo, sessions := newTestService(t)
	ctx := context.Background()
	boom := errors.New("disk full")
	sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{Topic: "caching"}, nil)
	repo.EXPECT().Create(ctx, domainexercise.Challenge{
		ID: "challenge-1", Type: domainexercise.ChallengeTypeArchitecture,
		Title: "Design a URL shortener", Scenario: "A startup needs a service that shortens URLs.",
		FunctionalRequirements: []string{"Create a short link"}, NonFunctionalRequirements: []string{}, Constraints: []string{},
		Objectives: []string{"Justify the storage choice"}, Difficulty: domainexercise.DifficultyBeginner,
		Topic: "caching", SourceSessionID: "session-1", Origin: domainexercise.OriginManual, TimeLimitMinutes: 30, DefenseQuestions: 5,
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}).Return(boom)

	// When creating the challenge
	_, err := service.Create(ctx, validInput())

	// Then the failure surfaces
	require.ErrorIs(t, err, boom)
}

func TestChallengeService_Get_returnsTheChallenge(t *testing.T) {
	// Given a stored challenge
	service, repo, _ := newTestService(t)
	ctx := context.Background()
	stored := domainexercise.Challenge{ID: "challenge-1", Title: "Stored"}
	repo.EXPECT().Get(ctx, "challenge-1").Return(stored, nil)

	// When getting it
	got, err := service.Get(ctx, "challenge-1")

	// Then it is returned unchanged
	require.NoError(t, err)
	assert.Equal(t, stored, got)
}

func TestChallengeService_Get_keepsNotFoundIdentifiable(t *testing.T) {
	// Given a repository without the challenge
	service, repo, _ := newTestService(t)
	ctx := context.Background()
	repo.EXPECT().Get(ctx, "missing").Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)

	// When getting it
	_, err := service.Get(ctx, "missing")

	// Then ErrChallengeNotFound can still be matched
	require.ErrorIs(t, err, domainexercise.ErrChallengeNotFound)
}

func TestChallengeService_List_delegatesFilterAndPage(t *testing.T) {
	// Given a repository answering one page
	service, repo, _ := newTestService(t)
	ctx := context.Background()
	filter := domainexercise.ChallengeFilter{Query: "cache", Difficulty: domainexercise.DifficultyAdvanced}
	page := domainexercise.Page{Limit: 10, Offset: 20}
	result := domainexercise.ChallengePage{Items: []domainexercise.Challenge{{ID: "challenge-1"}}, Total: 21}
	repo.EXPECT().List(ctx, filter, page).Return(result, nil)

	// When listing
	got, err := service.List(ctx, filter, page)

	// Then the page comes back unchanged
	require.NoError(t, err)
	assert.Equal(t, result, got)
}

func TestChallengeService_List_returnsRepositoryFailure(t *testing.T) {
	// Given a failing repository
	service, repo, _ := newTestService(t)
	ctx := context.Background()
	boom := errors.New("db down")
	repo.EXPECT().List(ctx, domainexercise.ChallengeFilter{}, domainexercise.Page{}).
		Return(domainexercise.ChallengePage{}, boom)

	// When listing
	_, err := service.List(ctx, domainexercise.ChallengeFilter{}, domainexercise.Page{})

	// Then the failure surfaces
	require.ErrorIs(t, err, boom)
}
