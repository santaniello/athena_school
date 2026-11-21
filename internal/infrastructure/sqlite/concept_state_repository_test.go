package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

var conceptBase = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func setupConceptFixture(t *testing.T) (*sql.DB, *ConceptStateRepository) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP);
		INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES
			('session-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP),
			('session-2', 'Rust', 'socratic', 'folder-1', CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
	return db, NewConceptStateRepository(db)
}

func concept(sessionID, name string, status domainstudy.ConceptStatus, age time.Duration) domainstudy.ConceptState {
	return domainstudy.ConceptState{
		SessionID:       sessionID,
		Name:            name,
		Status:          status,
		LastStrategy:    domainstudy.StrategyGuidedPractice,
		AssistanceCount: 1,
		EvidenceCount:   2,
		UpdatedAt:       conceptBase.Add(-age),
	}
}

func conceptNames(t *testing.T, repo *ConceptStateRepository, sessionID string) []string {
	t.Helper()
	states, err := repo.ListBySession(context.Background(), sessionID)
	require.NoError(t, err)
	names := make([]string, 0, len(states))
	for _, state := range states {
		names = append(names, state.Name)
	}
	return names
}

func TestConceptStateRepository_UpsertThenList_roundTripsEveryField(t *testing.T) {
	// Given a stored concept
	_, repo := setupConceptFixture(t)
	want := concept("session-1", "race condition", domainstudy.StatusPartiallyUnderstood, 0)
	require.NoError(t, repo.Upsert(context.Background(), want))

	// When listing the session's concepts
	got, err := repo.ListBySession(context.Background(), "session-1")

	// Then the row comes back unchanged, with its time in UTC
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, want, got[0])
}

func TestConceptStateRepository_Upsert_replacesTheRowOfTheSameConcept(t *testing.T) {
	// Given a stored concept
	_, repo := setupConceptFixture(t)
	require.NoError(t, repo.Upsert(context.Background(), concept("session-1", "mutex", domainstudy.StatusIntroduced, time.Hour)))

	// When upserting the same session and name with new values
	updated := concept("session-1", "mutex", domainstudy.StatusDemonstrated, 0)
	updated.AssistanceCount = 5
	require.NoError(t, repo.Upsert(context.Background(), updated))

	// Then there is still one row, with the new values
	got, err := repo.ListBySession(context.Background(), "session-1")
	require.NoError(t, err)
	assert.Equal(t, []domainstudy.ConceptState{updated}, got)
}

func TestConceptStateRepository_ListBySession_isScopedAndOrderedByUpdateTime(t *testing.T) {
	// Given concepts saved out of order, in two sessions
	_, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "newest", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "oldest", domainstudy.StatusIntroduced, 2*time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "middle", domainstudy.StatusIntroduced, time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-2", "other session", domainstudy.StatusIntroduced, 0)))

	// When listing session-1
	// Then only its concepts come back, oldest update first
	assert.Equal(t, []string{"oldest", "middle", "newest"}, conceptNames(t, repo, "session-1"))
	assert.Equal(t, []string{"other session"}, conceptNames(t, repo, "session-2"))
	assert.Empty(t, conceptNames(t, repo, "no-such-session"))
}

func TestConceptStateRepository_Upsert_rejectsAnUnknownStatus(t *testing.T) {
	// Given a concept with a status outside the five
	_, repo := setupConceptFixture(t)

	// When saving it
	err := repo.Upsert(context.Background(), concept("session-1", "x", domainstudy.ConceptStatus("MASTERED"), 0))

	// Then the database refuses it
	require.Error(t, err)
	assert.Empty(t, conceptNames(t, repo, "session-1"))
}

func TestConceptStateRepository_DeleteOldest_dropsTheOldestDemonstratedFirst(t *testing.T) {
	// Given an older non-demonstrated concept and two demonstrated ones
	_, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "oldest introduced", domainstudy.StatusIntroduced, 5*time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "old demonstrated", domainstudy.StatusDemonstrated, 3*time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "new demonstrated", domainstudy.StatusDemonstrated, time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "current", domainstudy.StatusIntroduced, 0)))

	// When evicting
	require.NoError(t, repo.DeleteOldest(ctx, "session-1", "current"))

	// Then the oldest demonstrated one is the one dropped, only that one
	assert.Equal(t, []string{"oldest introduced", "new demonstrated", "current"}, conceptNames(t, repo, "session-1"))
}

func TestConceptStateRepository_DeleteOldest_dropsTheOldestOverall_whenNoneIsDemonstrated(t *testing.T) {
	// Given concepts none of which is demonstrated
	_, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "newer", domainstudy.StatusNeedsReview, time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "oldest", domainstudy.StatusPartiallyUnderstood, 2*time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "current", domainstudy.StatusIntroduced, 0)))

	// When evicting
	require.NoError(t, repo.DeleteOldest(ctx, "session-1", "current"))

	// Then the oldest one goes
	assert.Equal(t, []string{"newer", "current"}, conceptNames(t, repo, "session-1"))
}

func TestConceptStateRepository_DeleteOldest_neverDropsTheKeptConcept(t *testing.T) {
	// Given the concept just written is the only DEMONSTRATED one (and the oldest by time)
	_, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "just demonstrated", domainstudy.StatusDemonstrated, 9*time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "other", domainstudy.StatusIntroduced, time.Hour)))

	// When evicting while keeping it
	require.NoError(t, repo.DeleteOldest(ctx, "session-1", "just demonstrated"))

	// Then another concept is dropped instead
	assert.Equal(t, []string{"just demonstrated"}, conceptNames(t, repo, "session-1"))
}

func TestConceptStateRepository_DeleteOldest_withOnlyTheKeptConcept_deletesNothing(t *testing.T) {
	// Given a session whose only concept is the kept one, and another session
	_, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "only", domainstudy.StatusDemonstrated, 0)))
	require.NoError(t, repo.Upsert(ctx, concept("session-2", "only", domainstudy.StatusDemonstrated, 0)))

	// When evicting
	require.NoError(t, repo.DeleteOldest(ctx, "session-1", "only"))

	// Then nothing is deleted, in either session
	assert.Equal(t, []string{"only"}, conceptNames(t, repo, "session-1"))
	assert.Equal(t, []string{"only"}, conceptNames(t, repo, "session-2"))
}

func TestConceptStateRepository_DeleteOldest_isScopedToTheSession(t *testing.T) {
	// Given an old concept in another session and a candidate in this one
	_, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-2", "elsewhere", domainstudy.StatusDemonstrated, 9*time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "candidate", domainstudy.StatusIntroduced, time.Hour)))
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "current", domainstudy.StatusIntroduced, 0)))

	// When evicting in session-1
	require.NoError(t, repo.DeleteOldest(ctx, "session-1", "current"))

	// Then session-2 is untouched
	assert.Equal(t, []string{"current"}, conceptNames(t, repo, "session-1"))
	assert.Equal(t, []string{"elsewhere"}, conceptNames(t, repo, "session-2"))
}

func TestConceptStateRepository_runsInsideATransaction(t *testing.T) {
	// Given a transaction that upserts and then fails
	db, repo := setupConceptFixture(t)
	transactor := NewSQLTransactor(db)

	// When it rolls back
	err := transactor.WithinTx(context.Background(), func(ctx context.Context) error {
		require.NoError(t, repo.Upsert(ctx, concept("session-1", "rolled back", domainstudy.StatusIntroduced, 0)))
		return assert.AnError
	})

	// Then the upsert did not persist
	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, conceptNames(t, repo, "session-1"))
}

func TestSessionDelete_removesItsConcepts(t *testing.T) {
	// Given concepts in two sessions
	db, repo := setupConceptFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Upsert(ctx, concept("session-1", "gone", domainstudy.StatusIntroduced, 0)))
	require.NoError(t, repo.Upsert(ctx, concept("session-2", "stays", domainstudy.StatusIntroduced, 0)))

	// When deleting session-1
	require.NoError(t, NewSessionRepository(db).Delete(ctx, "session-1"))

	// Then its concepts go with it, and the other session keeps its own
	assert.Empty(t, conceptNames(t, repo, "session-1"))
	assert.Equal(t, []string{"stays"}, conceptNames(t, repo, "session-2"))
}

func TestReset_clearsConcepts(t *testing.T) {
	// Given a stored concept
	db, repo := setupConceptFixture(t)
	require.NoError(t, repo.Upsert(context.Background(), concept("session-1", "gone", domainstudy.StatusIntroduced, 0)))

	// When resetting local data
	require.NoError(t, NewResetter(db).Reset(context.Background()))

	// Then the table is empty
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM session_concepts`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestOpen_createsSessionConceptsTable_onADatabaseThatPredatesIt(t *testing.T) {
	// Given a database opened before the session_concepts migration existed
	path := filepath.Join(t.TempDir(), "athena.db")
	first, err := Open(path)
	require.NoError(t, err)
	_, err = first.Exec(`DROP TABLE session_concepts`)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// When opening it through the current migration path
	db, err := Open(path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Then the table exists again
	var tableName string
	require.NoError(t, db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'session_concepts'`,
	).Scan(&tableName))
}
