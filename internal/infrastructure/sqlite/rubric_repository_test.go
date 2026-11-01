package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

func newTestRubricRepository(t *testing.T) (*RubricRepository, *sql.DB) {
	t.Helper()
	challenges, db := newTestChallengeRepository(t)
	require.NoError(t, challenges.Create(context.Background(), newTestChallenge("challenge-1")))
	return NewRubricRepository(db), db
}

func newTestRubricVersion(id string, version int) exercise.RubricVersion {
	criteria := exercise.BaseRubric()
	for i := range criteria {
		criteria[i].ID = id + "-base-" + criteria[i].Key
		criteria[i].Weight = 15
	}
	criteria = append(criteria, exercise.RubricCriterion{
		ID:          id + "-cache",
		Key:         "cache-policy",
		Name:        "Cache policy",
		Description: "The eviction policy fits the access pattern.",
		Weight:      25,
		Source:      exercise.CriterionSourceChallenge,
		Mandatory:   true,
		Levels:      [exercise.LevelCount]string{"none", "named", "chosen", "justified", "compared, with \"quotes\" and ünïcode"},
	})
	return exercise.RubricVersion{
		ID:          id,
		ChallengeID: "challenge-1",
		Version:     version,
		BaseVersion: exercise.BaseRubricVersion,
		Criteria:    criteria,
		CreatedAt:   time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC),
	}
}

func TestRubricRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given a stored challenge and a rubric with base and challenge criteria
	repo, _ := newTestRubricRepository(t)
	ctx := context.Background()
	version := newTestRubricVersion("rubric-1", 1)

	// When creating it and reading it back
	require.NoError(t, repo.Create(ctx, version))
	got, err := repo.Get(ctx, "rubric-1")

	// Then every field survives: order, weights, source, mandatory flag, levels and timestamp
	require.NoError(t, err)
	assert.Equal(t, version, got)
}

func TestRubricRepository_Get_survivesReopeningTheDatabase(t *testing.T) {
	// Given a rubric stored in a database file
	path := filepath.Join(t.TempDir(), "athena.db")
	db, err := Open(path)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-a', 'Folder A', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	insertTestSession(t, db, "session-1", "caching")
	ctx := context.Background()
	require.NoError(t, NewChallengeRepository(db).Create(ctx, newTestChallenge("challenge-1")))
	version := newTestRubricVersion("rubric-1", 1)
	require.NoError(t, NewRubricRepository(db).Create(ctx, version))
	require.NoError(t, db.Close())

	// When reopening the file
	reopened, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := NewRubricRepository(reopened).Get(ctx, "rubric-1")

	// Then the rubric is unchanged
	require.NoError(t, err)
	assert.Equal(t, version, got)
}

func TestRubricRepository_Get_unknownVersionReturnsNotFound(t *testing.T) {
	// Given an empty repository
	repo, _ := newTestRubricRepository(t)

	// When reading an unknown version id
	_, err := repo.Get(context.Background(), "missing")

	// Then ErrRubricNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrRubricNotFound)
}

func TestRubricRepository_Latest_returnsTheHighestVersion(t *testing.T) {
	// Given versions 1, 2 and 3 stored out of order
	repo, _ := newTestRubricRepository(t)
	ctx := context.Background()
	for _, version := range []exercise.RubricVersion{newTestRubricVersion("r2", 2), newTestRubricVersion("r10", 10), newTestRubricVersion("r1", 1)} {
		require.NoError(t, repo.Create(ctx, version))
	}

	// When asking for the latest
	got, err := repo.Latest(ctx, "challenge-1")

	// Then it is the highest version number, compared as a number (10 over 2)
	require.NoError(t, err)
	assert.Equal(t, "r10", got.ID)
	assert.Len(t, got.Criteria, 6)
}

func TestRubricRepository_Latest_challengeWithoutRubricReturnsNotFound(t *testing.T) {
	// Given a challenge with no rubric
	repo, _ := newTestRubricRepository(t)

	// When asking for its latest rubric
	_, err := repo.Latest(context.Background(), "challenge-1")

	// Then ErrRubricNotFound is returned
	assert.ErrorIs(t, err, exercise.ErrRubricNotFound)
}

func TestRubricRepository_ListVersions_returnsAllVersionsOldestFirst(t *testing.T) {
	// Given two versions of the challenge's rubric, created newest first
	repo, _ := newTestRubricRepository(t)
	ctx := context.Background()
	v2 := newTestRubricVersion("r2", 2)
	v1 := newTestRubricVersion("r1", 1)
	require.NoError(t, repo.Create(ctx, v2))
	require.NoError(t, repo.Create(ctx, v1))

	// When listing them
	got, err := repo.ListVersions(ctx, "challenge-1")

	// Then they come oldest first, each with its criteria
	require.NoError(t, err)
	assert.Equal(t, []exercise.RubricVersion{v1, v2}, got)
}

func TestRubricRepository_ListVersions_unknownChallengeReturnsEmptyList(t *testing.T) {
	// Given a repository with no rubric for the challenge
	repo, _ := newTestRubricRepository(t)

	// When listing its versions
	got, err := repo.ListVersions(context.Background(), "challenge-1")

	// Then the result is an empty, non-nil list
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestRubricRepository_Create_failureMidwayLeavesNothingStored(t *testing.T) {
	// Given a rubric whose second criterion repeats the first one's id
	repo, db := newTestRubricRepository(t)
	ctx := context.Background()
	version := newTestRubricVersion("rubric-1", 1)
	version.Criteria[1].ID = version.Criteria[0].ID

	// When creating it
	err := repo.Create(ctx, version)

	// Then it fails and neither the version nor its first criterion remains
	require.Error(t, err)
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM rubric_versions`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM rubric_criteria`))
}

func TestRubricRepository_Create_rejectsASecondRubricWithTheSameVersionNumber(t *testing.T) {
	// Given version 1 already stored
	repo, db := newTestRubricRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestRubricVersion("r1", 1)))

	// When creating another rubric as version 1 of the same challenge
	err := repo.Create(ctx, newTestRubricVersion("r1-again", 1))

	// Then it is rejected and the original is untouched
	require.Error(t, err)
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM rubric_versions`))
	assert.Equal(t, 6, countRows(t, db, `SELECT COUNT(*) FROM rubric_criteria`))
}

func TestRubricRepository_Create_rejectsAnUnknownChallenge(t *testing.T) {
	// Given a rubric for a challenge that does not exist
	repo, _ := newTestRubricRepository(t)
	version := newTestRubricVersion("rubric-1", 1)
	version.ChallengeID = "missing"

	// When creating it
	err := repo.Create(context.Background(), version)

	// Then the foreign key rejects it
	assert.Error(t, err)
}

func TestRubricRepository_Create_joinsAnEnclosingTransaction(t *testing.T) {
	// Given a repository and a transactor
	repo, db := newTestRubricRepository(t)
	ctx := context.Background()
	rollback := errors.New("rollback")

	// When creating a rubric inside a transaction that then fails
	err := NewSQLTransactor(db).WithinTx(ctx, func(ctx context.Context) error {
		require.NoError(t, repo.Create(ctx, newTestRubricVersion("rubric-1", 1)))
		return rollback
	})

	// Then the rubric is rolled back with the transaction
	require.ErrorIs(t, err, rollback)
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM rubric_versions`))
	assert.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM rubric_criteria`))
}

func TestRubricRepository_Create_commitsWithAnEnclosingTransaction(t *testing.T) {
	// Given a repository and a transactor
	repo, db := newTestRubricRepository(t)
	ctx := context.Background()

	// When creating a rubric inside a transaction that succeeds
	err := NewSQLTransactor(db).WithinTx(ctx, func(ctx context.Context) error {
		return repo.Create(ctx, newTestRubricVersion("rubric-1", 1))
	})

	// Then the rubric is stored
	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM rubric_versions`))
}

func TestRubricRepository_history_isNotRemovedBySideEffects(t *testing.T) {
	// Given a challenge that has a rubric
	repo, db := newTestRubricRepository(t)
	require.NoError(t, repo.Create(context.Background(), newTestRubricVersion("rubric-1", 1)))

	// When deleting the challenge, or a version that has criteria
	_, challengeErr := db.Exec(`DELETE FROM challenges WHERE id = 'challenge-1'`)
	_, versionErr := db.Exec(`DELETE FROM rubric_versions WHERE id = 'rubric-1'`)

	// Then neither cascades: both deletes are refused and the rubric is intact
	assert.Error(t, challengeErr)
	assert.Error(t, versionErr)
	assert.Equal(t, 1, countRows(t, db, `SELECT COUNT(*) FROM rubric_versions`))
	assert.Equal(t, 6, countRows(t, db, `SELECT COUNT(*) FROM rubric_criteria`))
}

func TestRubricRepository_Get_corruptLevelsReturnsAnError(t *testing.T) {
	// Given a stored criterion whose levels JSON does not hold five entries
	repo, db := newTestRubricRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestRubricVersion("rubric-1", 1)))
	_, err := db.Exec(`UPDATE rubric_criteria SET levels = '["only","two"]' WHERE position = 0`)
	require.NoError(t, err)

	// When reading the rubric
	_, err = repo.Get(ctx, "rubric-1")

	// Then the corruption is reported, not hidden
	assert.Error(t, err)
	assert.NotErrorIs(t, err, exercise.ErrRubricNotFound)
}
