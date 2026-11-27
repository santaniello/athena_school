package exercise

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type catalogFixture struct {
	service    *CatalogService
	challenges *domainmocks.MockChallengeRepository
	records    *domainmocks.MockAttemptRecordRepository
	profiles   *exercisemocks.MockProfileReader
}

func newCatalogFixture(t *testing.T) *catalogFixture {
	t.Helper()
	f := &catalogFixture{
		challenges: domainmocks.NewMockChallengeRepository(t),
		records:    domainmocks.NewMockAttemptRecordRepository(t),
		profiles:   exercisemocks.NewMockProfileReader(t),
	}
	f.service = NewCatalogService(f.challenges, f.records, f.profiles)
	return f
}

var catalogPage = domainexercise.Page{Limit: domainexercise.MaxPageLimit}

func catalogChallenge(id, title string) domainexercise.Challenge {
	return domainexercise.Challenge{
		ID: id, Title: title, Topic: "caching", Difficulty: domainexercise.DifficultyIntermediate,
		TimeLimitMinutes: 45, Scenario: "secret scenario",
	}
}

// expectScope makes the repository return the challenges for the scope filter.
func (f *catalogFixture) expectScope(filter domainexercise.ChallengeFilter, challenges ...domainexercise.Challenge) {
	f.challenges.EXPECT().List(context.Background(), filter, catalogPage).
		Return(domainexercise.ChallengePage{Items: challenges, Total: len(challenges)}, nil)
}

func recordIn(state domainexercise.AttemptState) domainexercise.AttemptRecord {
	return domainexercise.AttemptRecord{Attempt: domainexercise.ChallengeAttempt{ID: "a-" + string(state), State: state}}
}

// passedRecord is an attempt whose two evaluations pass under a minimum of 70.
func passedRecord(diagramScore int) domainexercise.AttemptRecord {
	rubric := domainexercise.RubricVersion{ID: "r", Criteria: []domainexercise.RubricCriterion{{ID: "c1", Weight: 100, Mandatory: true}}}
	results := []domainexercise.CriterionResult{{CriterionID: "c1", Level: 4}}
	return domainexercise.AttemptRecord{
		Attempt: domainexercise.ChallengeAttempt{
			ID: "a-passed", State: domainexercise.AttemptFinished, PassScore: 70,
			SubmittedAt: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC),
		},
		Diagram:       &domainexercise.Evaluation{Kind: domainexercise.EvaluationKindDiagram, Score: diagramScore, Results: results},
		Understanding: &domainexercise.Evaluation{Kind: domainexercise.EvaluationKindUnderstanding, Score: 90, Results: results},
		Rubric:        &rubric,
	}
}

func TestCatalogService_ListCatalog_describesEachChallengeWithItsDerivedProgress(t *testing.T) {
	// Given a challenge never tried and one passed with two attempts
	f := newCatalogFixture(t)
	f.expectScope(domainexercise.ChallengeFilter{}, catalogChallenge("c1", "Never tried"), catalogChallenge("c2", "Passed"))
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1", "c2"}).Return(map[string][]domainexercise.AttemptRecord{
		"c2": {passedRecord(86), recordIn(domainexercise.AttemptFinished)},
	}, nil)

	// When listing the catalog
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then each item carries only what does not give the exam away, plus its progress
	require.NoError(t, err)
	assert.Equal(t, []CatalogItem{
		{ChallengeID: "c1", Title: "Never tried", Topic: "caching", Difficulty: domainexercise.DifficultyIntermediate,
			TimeLimitMinutes: 45, Status: domainexercise.ProgressNotStarted},
		{ChallengeID: "c2", Title: "Passed", Topic: "caching", Difficulty: domainexercise.DifficultyIntermediate,
			TimeLimitMinutes: 45, Status: domainexercise.ProgressCompleted, Attempts: 2, BestDiagramScore: 86},
	}, catalog.Items)
}

func TestCatalogService_ListCatalog_countsTotalCompletedAndInProgress(t *testing.T) {
	// Given one challenge of each kind: completed, in progress, needs improvement, not started
	f := newCatalogFixture(t)
	f.expectScope(domainexercise.ChallengeFilter{},
		catalogChallenge("done", "d"), catalogChallenge("running", "r"), catalogChallenge("failed", "f"), catalogChallenge("new", "n"))
	failed := passedRecord(50)
	failed.Attempt.PassScore = 100
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"done", "running", "failed", "new"}).
		Return(map[string][]domainexercise.AttemptRecord{
			"done":    {passedRecord(90)},
			"running": {recordIn(domainexercise.AttemptInProgress)},
			"failed":  {failed},
		}, nil)

	// When listing the catalog
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then the counts add up what the statuses say
	require.NoError(t, err)
	assert.Equal(t, 4, catalog.Total)
	assert.Equal(t, 1, catalog.Completed)
	assert.Equal(t, 1, catalog.InProgress)
	statuses := map[string]domainexercise.ProgressStatus{}
	for _, item := range catalog.Items {
		statuses[item.ChallengeID] = item.Status
	}
	assert.Equal(t, map[string]domainexercise.ProgressStatus{
		"done": domainexercise.ProgressCompleted, "running": domainexercise.ProgressInProgress,
		"failed": domainexercise.ProgressNeedsImprovement, "new": domainexercise.ProgressNotStarted,
	}, statuses)
}

func TestCatalogService_ListCatalog_putsTheChallengeWithARunningAttemptFirst(t *testing.T) {
	// Given a completed challenge that has a new attempt running, listed after a newer untouched one
	f := newCatalogFixture(t)
	f.expectScope(domainexercise.ChallengeFilter{},
		catalogChallenge("newer", "n"), catalogChallenge("older", "o"), catalogChallenge("oldest", "x"))
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"newer", "older", "oldest"}).
		Return(map[string][]domainexercise.AttemptRecord{
			"older": {passedRecord(80), recordIn(domainexercise.AttemptInProgress)},
		}, nil)

	// When listing the catalog
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then the running one comes first and the rest keep the repository order
	require.NoError(t, err)
	assert.Equal(t, []string{"older", "newer", "oldest"}, catalogIDs(catalog.Items))
}

func catalogIDs(items []CatalogItem) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ChallengeID
	}
	return ids
}

func TestCatalogService_ListCatalog_filtersByStatusAfterDerivingIt(t *testing.T) {
	// Given a completed and a not started challenge
	f := newCatalogFixture(t)
	f.expectScope(domainexercise.ChallengeFilter{}, catalogChallenge("done", "d"), catalogChallenge("new", "n"))
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"done", "new"}).
		Return(map[string][]domainexercise.AttemptRecord{"done": {passedRecord(90)}}, nil)

	// When filtering by COMPLETED
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{Status: domainexercise.ProgressCompleted})

	// Then only the completed one is listed, and the counts follow the status filter
	require.NoError(t, err)
	assert.Equal(t, []string{"done"}, catalogIDs(catalog.Items))
	assert.Equal(t, 1, catalog.Total)
	assert.Equal(t, 1, catalog.Completed)
}

func TestCatalogService_ListCatalog_passesDifficultyAndTopicToTheRepository(t *testing.T) {
	// Given a repository that expects the scope filter
	f := newCatalogFixture(t)
	f.expectScope(domainexercise.ChallengeFilter{Topic: "caching", Difficulty: domainexercise.DifficultyBeginner})
	filter := CatalogFilter{Topic: "caching", Difficulty: domainexercise.DifficultyBeginner}

	// When listing the catalog
	catalog, err := f.service.ListCatalog(context.Background(), filter)

	// Then no challenge matches, and the attempts are not even read
	require.NoError(t, err)
	assert.NotNil(t, catalog.Items)
	assert.Empty(t, catalog.Items)
	assert.Zero(t, catalog.Total)
}

func TestCatalogService_ListCatalog_searchNarrowsTheListButNotTheCounts(t *testing.T) {
	// Given two challenges in scope, only one matching the text
	f := newCatalogFixture(t)
	f.expectScope(domainexercise.ChallengeFilter{}, catalogChallenge("c1", "Cache"), catalogChallenge("c2", "Queue"))
	f.expectScope(domainexercise.ChallengeFilter{Query: "cache"}, catalogChallenge("c1", "Cache"))
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1", "c2"}).
		Return(map[string][]domainexercise.AttemptRecord{}, nil)

	// When searching for "cache"
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{Query: "cache"})

	// Then only the match is listed while the counts still cover the scope
	require.NoError(t, err)
	assert.Equal(t, []string{"c1"}, catalogIDs(catalog.Items))
	assert.Equal(t, 2, catalog.Total)
}

func TestCatalogService_ListCatalog_readsEveryPageOfTheScope(t *testing.T) {
	// Given more challenges than one page holds
	f := newCatalogFixture(t)
	ctx := context.Background()
	first := make([]domainexercise.Challenge, domainexercise.MaxPageLimit)
	ids := make([]string, 0, domainexercise.MaxPageLimit+1)
	for i := range first {
		first[i] = catalogChallenge(fmt.Sprintf("c%03d", i), "t")
		ids = append(ids, first[i].ID)
	}
	last := catalogChallenge("last", "t")
	ids = append(ids, last.ID)
	f.challenges.EXPECT().List(ctx, domainexercise.ChallengeFilter{}, catalogPage).
		Return(domainexercise.ChallengePage{Items: first, Total: domainexercise.MaxPageLimit + 1}, nil)
	f.challenges.EXPECT().List(ctx, domainexercise.ChallengeFilter{}, domainexercise.Page{Limit: domainexercise.MaxPageLimit, Offset: domainexercise.MaxPageLimit}).
		Return(domainexercise.ChallengePage{Items: []domainexercise.Challenge{last}, Total: domainexercise.MaxPageLimit + 1}, nil)
	f.records.EXPECT().ListByChallenges(ctx, ids).Return(map[string][]domainexercise.AttemptRecord{}, nil)

	// When listing the catalog
	catalog, err := f.service.ListCatalog(ctx, CatalogFilter{})

	// Then all of them are listed
	require.NoError(t, err)
	assert.Equal(t, domainexercise.MaxPageLimit+1, catalog.Total)
	assert.Len(t, catalog.Items, domainexercise.MaxPageLimit+1)
}

func TestCatalogService_ListCatalog_failsWhenTheChallengesCannotBeListed(t *testing.T) {
	// Given a failing repository
	f := newCatalogFixture(t)
	boom := errors.New("boom")
	f.challenges.EXPECT().List(context.Background(), domainexercise.ChallengeFilter{}, catalogPage).Return(domainexercise.ChallengePage{}, boom)

	// When listing the catalog
	_, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then the error is wrapped
	require.ErrorIs(t, err, boom)
}

func TestCatalogService_ListCatalog_failsWhenTheSearchCannotBeListed(t *testing.T) {
	// Given a repository that fails only on the text search
	f := newCatalogFixture(t)
	boom := errors.New("boom")
	f.expectScope(domainexercise.ChallengeFilter{}, catalogChallenge("c1", "t"))
	f.challenges.EXPECT().List(context.Background(), domainexercise.ChallengeFilter{Query: "x"}, catalogPage).Return(domainexercise.ChallengePage{}, boom)

	// When searching
	_, err := f.service.ListCatalog(context.Background(), CatalogFilter{Query: "x"})

	// Then the error is wrapped
	require.ErrorIs(t, err, boom)
}

func TestCatalogService_ListCatalog_failsWhenTheAttemptsCannotBeRead(t *testing.T) {
	// Given a failing attempt store
	f := newCatalogFixture(t)
	boom := errors.New("boom")
	f.expectScope(domainexercise.ChallengeFilter{}, catalogChallenge("c1", "t"))
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1"}).Return(nil, boom)

	// When listing the catalog
	_, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then the error is wrapped
	require.ErrorIs(t, err, boom)
}

func TestCatalogService_ListCatalog_failsOnAnInconsistentAttempt(t *testing.T) {
	// Given an evaluated attempt without the minimum frozen on it
	f := newCatalogFixture(t)
	broken := passedRecord(80)
	broken.Attempt.PassScore = 0
	f.expectScope(domainexercise.ChallengeFilter{}, catalogChallenge("c1", "t"))
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1"}).
		Return(map[string][]domainexercise.AttemptRecord{"c1": {broken}}, nil)

	// When listing the catalog
	_, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then it is refused rather than shown with a guessed status
	require.Error(t, err)
}

func TestCatalogService_ListTopics_returnsTheDistinctTopics(t *testing.T) {
	// Given a repository with two topics
	f := newCatalogFixture(t)
	f.challenges.EXPECT().ListTopics(context.Background()).Return([]string{"caching", "queues"}, nil)

	// When listing the topics
	topics, err := f.service.ListTopics(context.Background())

	// Then they are returned as they are
	require.NoError(t, err)
	assert.Equal(t, []string{"caching", "queues"}, topics)
}

func TestCatalogService_ListTopics_wrapsRepositoryErrors(t *testing.T) {
	// Given a failing repository
	f := newCatalogFixture(t)
	boom := errors.New("boom")
	f.challenges.EXPECT().ListTopics(context.Background()).Return(nil, boom)

	// When listing the topics
	_, err := f.service.ListTopics(context.Background())

	// Then the error is wrapped
	require.ErrorIs(t, err, boom)
}

func TestCatalogService_ListCatalog_carriesTheKindAndTheUntimedLimit(t *testing.T) {
	// Given a practice with no time limit and an interview
	f := newCatalogFixture(t)
	practice := catalogChallenge("c1", "Cache aside")
	practice.Kind = domainexercise.ChallengeKindPractice
	practice.TimeLimitMinutes = 0
	interview := catalogChallenge("c2", "Design a shop")
	interview.Kind = domainexercise.ChallengeKindInterview
	f.expectScope(domainexercise.ChallengeFilter{}, practice, interview)
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1", "c2"}).
		Return(map[string][]domainexercise.AttemptRecord{}, nil)

	// When listing the catalog
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{})

	// Then each item says its kind and the practice has no time limit
	require.NoError(t, err)
	require.Len(t, catalog.Items, 2)
	assert.Equal(t, domainexercise.ChallengeKindPractice, catalog.Items[0].Kind)
	assert.Equal(t, 0, catalog.Items[0].TimeLimitMinutes)
	assert.Equal(t, domainexercise.ChallengeKindInterview, catalog.Items[1].Kind)
	assert.Equal(t, 45, catalog.Items[1].TimeLimitMinutes)
}

func canonicalChallenge(id, problemID string, difficulty domainexercise.Difficulty) domainexercise.Challenge {
	return domainexercise.Challenge{
		ID: id, Kind: domainexercise.ChallengeKindInterview, Title: "t", Topic: "x", Difficulty: difficulty,
		CanonicalProblemID: problemID, Scenario: "secret scenario",
	}
}

func TestCatalogService_ListCatalog_filtersByKindInScopeAndSearch(t *testing.T) {
	// Given a practice-only catalog with a text search
	f := newCatalogFixture(t)
	scope := domainexercise.ChallengeFilter{Kind: domainexercise.ChallengeKindPractice}
	search := domainexercise.ChallengeFilter{Query: "cache", Kind: domainexercise.ChallengeKindPractice}
	practice := catalogChallenge("c1", "Cache practice")
	f.expectScope(scope, practice)
	f.expectScope(search, practice)
	f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1"}).Return(nil, nil)

	// When listing it
	catalog, err := f.service.ListCatalog(context.Background(), CatalogFilter{Kind: domainexercise.ChallengeKindPractice, Query: "cache"})

	// Then the kind is part of both repository filters
	require.NoError(t, err)
	require.Len(t, catalog.Items, 1)
}

func TestCatalogService_ListCanonicalProblems_listsEverySixProblemsInOrderWithNoChallengeGenerated(t *testing.T) {
	// Given a fresh install: no canonical challenge exists and the profile is Portuguese
	f := newCatalogFixture(t)
	ctx := context.Background()
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{Language: "pt"}, nil)
	f.challenges.EXPECT().ListCanonical(ctx).Return(nil, nil)

	// When listing the problems (no attempt read is needed: strict mocks)
	items, err := f.service.ListCanonicalProblems(ctx)

	// Then all six come in order, named in the assistant language, with nothing generated
	require.NoError(t, err)
	require.Len(t, items, 6)
	assert.Equal(t, "url-shortener", items[0].ID)
	assert.Equal(t, domainexercise.DifficultyBeginner, items[0].Level)
	assert.Equal(t, "Encurtador de URL", items[0].Name)
	assert.Equal(t, "Encurte URLs longas e redirecione os links curtos.", items[0].Summary)
	assert.Equal(t, 0, items[0].Attempts)
	assert.False(t, items[0].HasHistory)
	assert.Equal(t, []DifficultyState{
		{Difficulty: domainexercise.DifficultyBeginner},
		{Difficulty: domainexercise.DifficultyIntermediate},
		{Difficulty: domainexercise.DifficultyAdvanced},
	}, items[0].Difficulties)
}

func TestCatalogService_ListCanonicalProblems_derivesTheStatusAndCountsOfEachDifficulty(t *testing.T) {
	// Given a problem with a passed beginner challenge (two attempts, one finished) and an untouched advanced one
	f := newCatalogFixture(t)
	ctx := context.Background()
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{Language: "en"}, nil)
	beginner := canonicalChallenge("c-b", "rate-limiter", domainexercise.DifficultyBeginner)
	advanced := canonicalChallenge("c-a", "rate-limiter", domainexercise.DifficultyAdvanced)
	f.challenges.EXPECT().ListCanonical(ctx).Return([]domainexercise.Challenge{beginner, advanced}, nil)
	f.records.EXPECT().ListByChallenges(ctx, []string{"c-b", "c-a"}).Return(map[string][]domainexercise.AttemptRecord{
		"c-b": {recordIn(domainexercise.AttemptInProgress), passedRecord(88)},
	}, nil)

	// When listing the problems
	items, err := f.service.ListCanonicalProblems(ctx)

	// Then the status is derived in the backend and the totals follow the attempts
	require.NoError(t, err)
	limiter := items[1]
	assert.Equal(t, "rate-limiter", limiter.ID)
	assert.Equal(t, "Rate limiter", limiter.Name)
	assert.Equal(t, 2, limiter.Attempts)
	assert.True(t, limiter.HasHistory)
	assert.Equal(t, []DifficultyState{
		{Difficulty: domainexercise.DifficultyBeginner, Generated: true, ChallengeID: "c-b", Status: domainexercise.ProgressCompleted},
		{Difficulty: domainexercise.DifficultyIntermediate},
		{Difficulty: domainexercise.DifficultyAdvanced, Generated: true, ChallengeID: "c-a", Status: domainexercise.ProgressNotStarted},
	}, limiter.Difficulties)
	assert.False(t, items[0].HasHistory)
}

func TestCatalogService_ListCanonicalProblems_hasNoHistoryWhileEveryAttemptIsUnfinished(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{Language: "en"}, nil)
	f.challenges.EXPECT().ListCanonical(ctx).Return([]domainexercise.Challenge{canonicalChallenge("c1", "url-shortener", domainexercise.DifficultyBeginner)}, nil)
	f.records.EXPECT().ListByChallenges(ctx, []string{"c1"}).Return(map[string][]domainexercise.AttemptRecord{
		"c1": {recordIn(domainexercise.AttemptInProgress)},
	}, nil)

	items, err := f.service.ListCanonicalProblems(ctx)

	require.NoError(t, err)
	assert.False(t, items[0].HasHistory)
	assert.Equal(t, 1, items[0].Attempts)
}

func TestCatalogService_ListCanonicalProblems_failsWhenAReadFails(t *testing.T) {
	boom := errors.New("boom")
	t.Run("profile", func(t *testing.T) {
		f := newCatalogFixture(t)
		f.profiles.EXPECT().ReadProfile(context.Background()).Return(domainexercise.GenerationProfile{}, boom)

		_, err := f.service.ListCanonicalProblems(context.Background())

		assert.ErrorIs(t, err, boom)
	})
	t.Run("challenges", func(t *testing.T) {
		f := newCatalogFixture(t)
		f.profiles.EXPECT().ReadProfile(context.Background()).Return(domainexercise.GenerationProfile{}, nil)
		f.challenges.EXPECT().ListCanonical(context.Background()).Return(nil, boom)

		_, err := f.service.ListCanonicalProblems(context.Background())

		assert.ErrorIs(t, err, boom)
	})
	t.Run("attempts", func(t *testing.T) {
		f := newCatalogFixture(t)
		f.profiles.EXPECT().ReadProfile(context.Background()).Return(domainexercise.GenerationProfile{}, nil)
		f.challenges.EXPECT().ListCanonical(context.Background()).Return([]domainexercise.Challenge{canonicalChallenge("c1", "url-shortener", domainexercise.DifficultyBeginner)}, nil)
		f.records.EXPECT().ListByChallenges(context.Background(), []string{"c1"}).Return(nil, boom)

		_, err := f.service.ListCanonicalProblems(context.Background())

		assert.ErrorIs(t, err, boom)
	})
}
