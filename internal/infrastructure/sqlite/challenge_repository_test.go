package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

func newTestChallengeRepository(t *testing.T) (*ChallengeRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-a', 'Folder A', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	insertTestSession(t, db, "session-1", "caching")
	return NewChallengeRepository(db), db
}

func insertTestSession(t *testing.T, db *sql.DB, id, topic string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES (?, ?, 'study', 'folder-a', CURRENT_TIMESTAMP)`,
		id, topic,
	)
	require.NoError(t, err)
}

func newTestChallenge(id string) exercise.Challenge {
	at := time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC)
	return exercise.Challenge{
		ID:                        id,
		Type:                      exercise.ChallengeTypeArchitecture,
		Title:                     "Design a URL shortener",
		Scenario:                  "A startup needs a service that shortens URLs.",
		FunctionalRequirements:    []string{"Create a short link", "Redirect a short link"},
		NonFunctionalRequirements: []string{"99.9% availability", "p99 under 100ms"},
		Constraints:               []string{"Single region", "Small team"},
		Objectives:                []string{"Justify the storage choice", "Explain the cache"},
		Difficulty:                exercise.DifficultyIntermediate,
		Topic:                     "caching",
		SourceSessionID:           "session-1",
		Origin:                    exercise.OriginAIGenerated,
		CreatedAt:                 at,
		UpdatedAt:                 at,
	}
}

func TestChallengeRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given a repository and a challenge with every list populated
	repo, _ := newTestChallengeRepository(t)
	ctx := context.Background()
	challenge := newTestChallenge("challenge-1")

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, challenge))
	got, err := repo.Get(ctx, "challenge-1")

	// Then every field, including list order and timestamps, is preserved
	require.NoError(t, err)
	assert.Equal(t, challenge, got)
}

func TestChallengeRepository_CreateThenGet_returnsEmptyListsWhenOptionalListsAreNil(t *testing.T) {
	// Given a challenge without optional lists
	repo, _ := newTestChallengeRepository(t)
	ctx := context.Background()
	challenge := newTestChallenge("challenge-1")
	challenge.NonFunctionalRequirements = nil
	challenge.Constraints = nil

	// When creating and reading it back
	require.NoError(t, repo.Create(ctx, challenge))
	got, err := repo.Get(ctx, "challenge-1")

	// Then the optional lists are empty, not nil
	require.NoError(t, err)
	assert.Equal(t, []string{}, got.NonFunctionalRequirements)
	assert.Equal(t, []string{}, got.Constraints)
}

func TestChallengeRepository_Create_storesNoSourceSessionWhenEmpty(t *testing.T) {
	// Given a challenge with no source session
	repo, db := newTestChallengeRepository(t)
	ctx := context.Background()
	challenge := newTestChallenge("challenge-1")
	challenge.SourceSessionID = ""

	// When creating it
	require.NoError(t, repo.Create(ctx, challenge))

	// Then the column is NULL and the challenge reads back with an empty id
	var raw sql.NullString
	require.NoError(t, db.QueryRow(`SELECT source_session_id FROM challenges WHERE id = 'challenge-1'`).Scan(&raw))
	assert.False(t, raw.Valid)
	got, err := repo.Get(ctx, "challenge-1")
	require.NoError(t, err)
	assert.Empty(t, got.SourceSessionID)
}

func TestChallengeRepository_Create_failsForUnknownSourceSession(t *testing.T) {
	// Given a challenge pointing at a session that does not exist
	repo, _ := newTestChallengeRepository(t)
	challenge := newTestChallenge("challenge-1")
	challenge.SourceSessionID = "missing-session"

	// When creating it
	err := repo.Create(context.Background(), challenge)

	// Then the foreign key rejects it
	assert.Error(t, err)
}

func TestChallengeRepository_Create_failsForDuplicateID(t *testing.T) {
	// Given a stored challenge
	repo, _ := newTestChallengeRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestChallenge("challenge-1")))

	// When creating another with the same id
	err := repo.Create(ctx, newTestChallenge("challenge-1"))

	// Then it fails instead of overwriting
	assert.Error(t, err)
}

func TestChallengeRepository_Get_returnsNotFoundForUnknownID(t *testing.T) {
	// Given an empty repository
	repo, _ := newTestChallengeRepository(t)

	// When getting an unknown id
	_, err := repo.Get(context.Background(), "missing")

	// Then ErrChallengeNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrChallengeNotFound)
}

func createChallenges(t *testing.T, repo *ChallengeRepository, challenges ...exercise.Challenge) {
	t.Helper()
	for _, challenge := range challenges {
		require.NoError(t, repo.Create(context.Background(), challenge))
	}
}

func challengeIDs(items []exercise.Challenge) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestChallengeRepository_List_ordersByUpdatedAtDescThenID(t *testing.T) {
	// Given challenges with distinct and equal update times, created out of order
	repo, _ := newTestChallengeRepository(t)
	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	withTime := func(id string, at time.Time) exercise.Challenge {
		c := newTestChallenge(id)
		c.UpdatedAt = at
		return c
	}
	createChallenges(t, repo,
		withTime("b", base),
		withTime("newest", base.Add(time.Hour)),
		withTime("a", base),
		withTime("oldest", base.Add(-time.Hour)),
	)

	// When listing without filters
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{}, exercise.Page{})

	// Then they come newest first, ties broken by id
	require.NoError(t, err)
	assert.Equal(t, []string{"newest", "a", "b", "oldest"}, challengeIDs(page.Items))
	assert.Equal(t, 4, page.Total)
}

func TestChallengeRepository_List_ordersSubSecondUpdatesCorrectly(t *testing.T) {
	// Given two challenges updated within the same second, the later one with trailing-zero nanoseconds
	repo, _ := newTestChallengeRepository(t)
	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	early := newTestChallenge("early")
	early.UpdatedAt = base.Add(100 * time.Millisecond)
	late := newTestChallenge("late")
	late.UpdatedAt = base.Add(500 * time.Millisecond)
	createChallenges(t, repo, early, late)

	// When listing
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{}, exercise.Page{})

	// Then the later update is first
	require.NoError(t, err)
	assert.Equal(t, []string{"late", "early"}, challengeIDs(page.Items))
}

func TestChallengeRepository_List_returnsEmptyPageWhenNothingMatches(t *testing.T) {
	// Given an empty repository
	repo, _ := newTestChallengeRepository(t)

	// When listing
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{}, exercise.Page{})

	// Then the page is empty with a zero total
	require.NoError(t, err)
	assert.Empty(t, page.Items)
	assert.NotNil(t, page.Items)
	assert.Zero(t, page.Total)
}

func TestChallengeRepository_List_filtersByQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"matches title case-insensitively", "SHORTENER", []string{"title"}},
		{"matches scenario", "payments", []string{"scenario"}},
		{"matches topic", "kubernetes", []string{"topic"}},
		{"ignores surrounding whitespace", "  shortener ", []string{"title"}},
		{"percent is literal", "50%", []string{"percent"}},
		{"underscore is literal", "a_b", []string{"underscore"}},
		{"backslash is literal", `c\d`, []string{"backslash"}},
		{"no match", "zzz", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given challenges whose text differs in title, scenario, topic and wildcard characters
			repo, _ := newTestChallengeRepository(t)
			build := func(id string, edit func(*exercise.Challenge)) exercise.Challenge {
				c := newTestChallenge(id)
				c.Title, c.Scenario, c.Topic = "plain title", "plain scenario", "plain topic"
				edit(&c)
				return c
			}
			createChallenges(t, repo,
				build("title", func(c *exercise.Challenge) { c.Title = "Design a URL shortener" }),
				build("scenario", func(c *exercise.Challenge) { c.Scenario = "A bank moving Payments online" }),
				build("topic", func(c *exercise.Challenge) { c.Topic = "kubernetes" }),
				build("percent", func(c *exercise.Challenge) { c.Scenario = "Reduce cost by 50% overall" }),
				build("underscore", func(c *exercise.Challenge) { c.Title = "Table a_b design" }),
				build("backslash", func(c *exercise.Challenge) { c.Title = `Path c\d design` }),
				build("decoy", func(c *exercise.Challenge) { c.Title, c.Scenario = "aXb 50 cXd", "other" }),
			)

			// When listing with the query
			page, err := repo.List(context.Background(), exercise.ChallengeFilter{Query: tt.query}, exercise.Page{})

			// Then only matching challenges are returned
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, challengeIDs(page.Items))
			assert.Equal(t, len(tt.want), page.Total)
		})
	}
}

func TestChallengeRepository_List_filtersByNormalizedTopic(t *testing.T) {
	// Given challenges with different stored topics
	repo, _ := newTestChallengeRepository(t)
	cache := newTestChallenge("cache")
	cache.Topic = "caching"
	queue := newTestChallenge("queue")
	queue.Topic = "message queues"
	createChallenges(t, repo, cache, queue)

	// When filtering by a topic with different case and whitespace
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{Topic: "  Caching "}, exercise.Page{})

	// Then the match is exact on the normalized snapshot
	require.NoError(t, err)
	assert.Equal(t, []string{"cache"}, challengeIDs(page.Items))
}

func TestChallengeRepository_List_topicFilterIsNotAPrefixMatch(t *testing.T) {
	// Given a challenge whose topic merely starts with the filter text
	repo, _ := newTestChallengeRepository(t)
	other := newTestChallenge("other")
	other.Topic = "caching strategies"
	createChallenges(t, repo, other)

	// When filtering by the shorter topic
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{Topic: "caching"}, exercise.Page{})

	// Then nothing matches
	require.NoError(t, err)
	assert.Empty(t, page.Items)
}

func TestChallengeRepository_List_filtersByDifficultyAndOrigin(t *testing.T) {
	// Given challenges across difficulties and origins
	repo, _ := newTestChallengeRepository(t)
	build := func(id string, d exercise.Difficulty, o exercise.ChallengeOrigin) exercise.Challenge {
		c := newTestChallenge(id)
		c.Difficulty, c.Origin = d, o
		return c
	}
	createChallenges(t, repo,
		build("beginner-manual", exercise.DifficultyBeginner, exercise.OriginManual),
		build("advanced-manual", exercise.DifficultyAdvanced, exercise.OriginManual),
		build("advanced-ai", exercise.DifficultyAdvanced, exercise.OriginAIGenerated),
	)
	ctx := context.Background()

	// When filtering by difficulty only
	byDifficulty, err := repo.List(ctx, exercise.ChallengeFilter{Difficulty: exercise.DifficultyAdvanced}, exercise.Page{})
	require.NoError(t, err)
	// And by origin only
	byOrigin, err := repo.List(ctx, exercise.ChallengeFilter{Origin: exercise.OriginManual}, exercise.Page{})
	require.NoError(t, err)

	// Then each filter narrows on its own column
	assert.ElementsMatch(t, []string{"advanced-manual", "advanced-ai"}, challengeIDs(byDifficulty.Items))
	assert.ElementsMatch(t, []string{"beginner-manual", "advanced-manual"}, challengeIDs(byOrigin.Items))
}

func TestChallengeRepository_List_combinesFiltersWithAnd(t *testing.T) {
	// Given challenges where each filter alone matches several
	repo, _ := newTestChallengeRepository(t)
	build := func(id, topic string, d exercise.Difficulty, o exercise.ChallengeOrigin, title string) exercise.Challenge {
		c := newTestChallenge(id)
		c.Topic, c.Difficulty, c.Origin, c.Title = topic, d, o, title
		return c
	}
	createChallenges(t, repo,
		build("hit", "caching", exercise.DifficultyAdvanced, exercise.OriginManual, "Edge cache"),
		build("wrong-topic", "queues", exercise.DifficultyAdvanced, exercise.OriginManual, "Edge cache"),
		build("wrong-difficulty", "caching", exercise.DifficultyBeginner, exercise.OriginManual, "Edge cache"),
		build("wrong-origin", "caching", exercise.DifficultyAdvanced, exercise.OriginAIGenerated, "Edge cache"),
		build("wrong-query", "caching", exercise.DifficultyAdvanced, exercise.OriginManual, "Other"),
	)

	// When listing with all four filters
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{
		Query: "edge", Topic: "caching", Difficulty: exercise.DifficultyAdvanced, Origin: exercise.OriginManual,
	}, exercise.Page{})

	// Then only the challenge satisfying all of them is returned
	require.NoError(t, err)
	assert.Equal(t, []string{"hit"}, challengeIDs(page.Items))
	assert.Equal(t, 1, page.Total)
}

func TestChallengeRepository_List_paginatesWithTotalOfAllMatches(t *testing.T) {
	// Given five challenges in a known order
	repo, _ := newTestChallengeRepository(t)
	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	for i, id := range []string{"c1", "c2", "c3", "c4", "c5"} {
		c := newTestChallenge(id)
		c.UpdatedAt = base.Add(-time.Duration(i) * time.Minute)
		createChallenges(t, repo, c)
	}
	ctx := context.Background()

	// When reading the second and the last page of size two
	second, err := repo.List(ctx, exercise.ChallengeFilter{}, exercise.Page{Limit: 2, Offset: 2})
	require.NoError(t, err)
	last, err := repo.List(ctx, exercise.ChallengeFilter{}, exercise.Page{Limit: 2, Offset: 4})
	require.NoError(t, err)

	// Then each page holds its slice and the total counts every match
	assert.Equal(t, []string{"c3", "c4"}, challengeIDs(second.Items))
	assert.Equal(t, 5, second.Total)
	assert.Equal(t, []string{"c5"}, challengeIDs(last.Items))
	assert.Equal(t, 5, last.Total)
}

func TestChallengeRepository_List_normalizesAnInvalidPage(t *testing.T) {
	// Given a stored challenge
	repo, _ := newTestChallengeRepository(t)
	createChallenges(t, repo, newTestChallenge("c1"))

	// When listing with a negative limit and offset
	page, err := repo.List(context.Background(), exercise.ChallengeFilter{}, exercise.Page{Limit: -1, Offset: -5})

	// Then the defaults apply instead of an error or an empty page
	require.NoError(t, err)
	assert.Equal(t, []string{"c1"}, challengeIDs(page.Items))
}
