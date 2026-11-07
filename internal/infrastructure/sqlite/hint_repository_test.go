package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

var hintTestTime = time.Date(2026, 11, 7, 9, 15, 0, 123456789, time.UTC)

// newTestHintRepository returns a repository over a database that holds an
// in-progress attempt-1.
func newTestHintRepository(t *testing.T) (*HintRepository, *AttemptRepository, *sql.DB) {
	t.Helper()
	attempts, db := newTestAttemptRepository(t)
	require.NoError(t, attempts.Create(context.Background(), newTestAttempt("attempt-1")))
	return NewHintRepository(db), attempts, db
}

func newTestHint(attemptID string, position int) exercise.Hint {
	return exercise.Hint{
		ID:        fmt.Sprintf("%s-hint-%d", attemptID, position),
		AttemptID: attemptID,
		Position:  position,
		Content:   fmt.Sprintf("What if \"%d\" fails? ünïcode", position),
		CreatedAt: hintTestTime,
	}
}

func TestHintRepository_AddThenListByAttempt_roundTripsEveryFieldInOrder(t *testing.T) {
	// Given two hints added out of order
	repo, _, _ := newTestHintRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Add(ctx, newTestHint("attempt-1", 1)))
	require.NoError(t, repo.Add(ctx, newTestHint("attempt-1", 2)))

	// When listing them
	got, err := repo.ListByAttempt(ctx, "attempt-1")

	// Then every field is preserved, in the order given
	require.NoError(t, err)
	assert.Equal(t, []exercise.Hint{newTestHint("attempt-1", 1), newTestHint("attempt-1", 2)}, got)
}

func TestHintRepository_ListByAttempt_returnsOnlyThatAttemptsHints(t *testing.T) {
	// Given hints on two attempts
	repo, _, db := newTestHintRepository(t)
	ctx := context.Background()
	insertFinishedAttempt(t, db, "finished-1")
	require.NoError(t, repo.Add(ctx, newTestHint("attempt-1", 1)))
	require.NoError(t, repo.Add(ctx, newTestHint("finished-1", 1)))

	// When listing one attempt
	got, err := repo.ListByAttempt(ctx, "finished-1")

	// Then only its hint comes back
	require.NoError(t, err)
	assert.Equal(t, []exercise.Hint{newTestHint("finished-1", 1)}, got)
}

func TestHintRepository_ListByAttempt_isEmptyWithoutHints(t *testing.T) {
	// Given an attempt without hints
	repo, _, _ := newTestHintRepository(t)

	// When listing them
	got, err := repo.ListByAttempt(context.Background(), "attempt-1")

	// Then the list is empty and not nil
	require.NoError(t, err)
	assert.Equal(t, []exercise.Hint{}, got)
}

func TestHintRepository_Add_refusesAFourthHint(t *testing.T) {
	// Given an attempt that already has MaxHints hints
	repo, _, db := newTestHintRepository(t)
	ctx := context.Background()
	for position := 1; position <= exercise.MaxHints; position++ {
		require.NoError(t, repo.Add(ctx, newTestHint("attempt-1", position)))
	}

	// When adding another one
	err := repo.Add(ctx, newTestHint("attempt-1", exercise.MaxHints+1))

	// Then it is refused and nothing is stored
	assert.ErrorIs(t, err, exercise.ErrNoHintsLeft)
	assert.Equal(t, exercise.MaxHints, countTableRows(t, db, "attempt_hints"))
}

func TestHintRepository_Add_refusesARepeatedPosition(t *testing.T) {
	// Given a stored first hint
	repo, _, db := newTestHintRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Add(ctx, newTestHint("attempt-1", 1)))

	// When another call stored the same position
	other := newTestHint("attempt-1", 1)
	other.ID = "other"
	err := repo.Add(ctx, other)

	// Then it is refused as a concurrent request
	assert.ErrorIs(t, err, exercise.ErrHintInProgress)
	assert.Equal(t, 1, countTableRows(t, db, "attempt_hints"))
}

func TestHintRepository_Add_underConcurrencyStoresAtMostMaxHints(t *testing.T) {
	// Given many concurrent adds, each for a distinct position
	repo, _, db := newTestHintRepository(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for position := 1; position <= exercise.MaxHints+4; position++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = repo.Add(ctx, newTestHint("attempt-1", position))
		}()
	}

	// When they all finish
	wg.Wait()

	// Then the cap held
	assert.Equal(t, exercise.MaxHints, countTableRows(t, db, "attempt_hints"))
}

func TestHintRepository_Add_failsForAnUnknownAttempt(t *testing.T) {
	// Given no attempt-9
	repo, _, db := newTestHintRepository(t)

	// When adding a hint to it
	err := repo.Add(context.Background(), newTestHint("attempt-9", 1))

	// Then the foreign key refuses it
	assert.Error(t, err)
	assert.Equal(t, 0, countTableRows(t, db, "attempt_hints"))
}

func TestAttemptRepository_DeleteUnfinished_removesTheAttemptsHints(t *testing.T) {
	// Given an in-progress attempt with a hint
	hints, attempts, db := newTestHintRepository(t)
	ctx := context.Background()
	require.NoError(t, hints.Add(ctx, newTestHint("attempt-1", 1)))

	// When discarding the attempt
	require.NoError(t, attempts.DeleteUnfinished(ctx, "attempt-1"))

	// Then its hints are gone
	assert.Equal(t, 0, countTableRows(t, db, "attempt_hints"))
}

func TestAttemptRepository_DeleteAllUnfinished_removesHintsButKeepsFinishedAttemptsHints(t *testing.T) {
	// Given a hint on a running attempt and one on a finished attempt
	hints, attempts, db := newTestHintRepository(t)
	ctx := context.Background()
	insertFinishedAttempt(t, db, "finished-1")
	require.NoError(t, hints.Add(ctx, newTestHint("attempt-1", 1)))
	require.NoError(t, hints.Add(ctx, newTestHint("finished-1", 1)))

	// When removing every unfinished attempt
	_, err := attempts.DeleteAllUnfinished(ctx)

	// Then only the finished attempt keeps its hint
	require.NoError(t, err)
	got, err := hints.ListByAttempt(ctx, "finished-1")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, 1, countTableRows(t, db, "attempt_hints"))
}

func TestAttemptRepository_DeleteUnfinishedEvaluations_removesTheAttemptsHints(t *testing.T) {
	// Given a failed-evaluation attempt with a hint
	hints, attempts, db := newTestHintRepository(t)
	ctx := context.Background()
	require.NoError(t, hints.Add(ctx, newTestHint("attempt-1", 1)))
	submitted, submission := submitForTest(t, newTestAttempt("attempt-1"), "submission-1")
	require.NoError(t, attempts.Submit(ctx, submitted, submission))
	require.NoError(t, attempts.StartEvaluation(ctx, "attempt-1"))

	// When deleting the unfinished evaluations
	removed, err := attempts.DeleteUnfinishedEvaluations(ctx)

	// Then the hint goes with its attempt
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, countTableRows(t, db, "attempt_hints"))
}

func TestHintRepository_Add_joinsTheTransactionOnTheContext(t *testing.T) {
	// Given a transaction that fails after adding a hint
	repo, _, db := newTestHintRepository(t)
	tx := NewSQLTransactor(db)
	sentinel := fmt.Errorf("boom")

	// When it rolls back
	err := tx.WithinTx(context.Background(), func(ctx context.Context) error {
		require.NoError(t, repo.Add(ctx, newTestHint("attempt-1", 1)))
		return sentinel
	})

	// Then the hint was not stored
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, 0, countTableRows(t, db, "attempt_hints"))
}

func TestHintRepository_Add_refusesAPositionOutsideTheRange(t *testing.T) {
	// Given an empty attempt
	repo, _, db := newTestHintRepository(t)

	// When adding a hint at a position beyond MaxHints
	err := repo.Add(context.Background(), newTestHint("attempt-1", exercise.MaxHints+1))

	// Then the database refuses it
	assert.Error(t, err)
	assert.Equal(t, 0, countTableRows(t, db, "attempt_hints"))
}
