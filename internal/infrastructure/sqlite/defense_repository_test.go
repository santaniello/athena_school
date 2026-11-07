package sqlite

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

var defenseTestTime = time.Date(2026, 11, 7, 9, 15, 0, 123456789, time.UTC)

func newTestDefenseTurn(attemptID string, position int) exercise.DefenseTurn {
	return exercise.DefenseTurn{
		ID:          fmt.Sprintf("%s-turn-%d", attemptID, position),
		AttemptID:   attemptID,
		Position:    position,
		CriterionID: "rubric-1-base-scalability",
		Question:    fmt.Sprintf("Why did you pick \"%d\" nodes? ünïcode", position),
		AskedAt:     defenseTestTime,
	}
}

// newTestDefenseRepository returns a repository over a database that holds a
// defending attempt-1.
func newTestDefenseRepository(t *testing.T) (*DefenseRepository, *AttemptRepository) {
	t.Helper()
	attempts, db := newTestAttemptRepository(t)
	attemptInState(t, attempts, "attempt-1", exercise.AttemptDefending)
	return NewDefenseRepository(db), attempts
}

func TestDefenseRepository_AddQuestionThenListByAttempt_roundTripsEveryFieldInOrder(t *testing.T) {
	// Given two questions added out of order
	repo, _ := newTestDefenseRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.AddQuestion(ctx, newTestDefenseTurn("attempt-1", 2)))
	require.NoError(t, repo.AddQuestion(ctx, newTestDefenseTurn("attempt-1", 1)))

	// When listing them
	got, err := repo.ListByAttempt(ctx, "attempt-1")

	// Then every field is preserved, unanswered, in the order asked
	require.NoError(t, err)
	assert.Equal(t, []exercise.DefenseTurn{newTestDefenseTurn("attempt-1", 1), newTestDefenseTurn("attempt-1", 2)}, got)
}

func TestDefenseRepository_ListByAttempt_returnsNoTurnsForAnAttemptWithoutQuestions(t *testing.T) {
	// Given a defending attempt with no question yet
	repo, _ := newTestDefenseRepository(t)

	// When listing its turns
	got, err := repo.ListByAttempt(context.Background(), "attempt-1")

	// Then the list is empty, not nil
	require.NoError(t, err)
	assert.Equal(t, []exercise.DefenseTurn{}, got)
}

func TestDefenseRepository_ListByAttempt_returnsOnlyThatAttemptsTurns(t *testing.T) {
	// Given turns on two attempts
	repo, attempts := newTestDefenseRepository(t)
	ctx := context.Background()
	attemptInState(t, attempts, "attempt-2", exercise.AttemptDefending)
	require.NoError(t, repo.AddQuestion(ctx, newTestDefenseTurn("attempt-1", 1)))
	require.NoError(t, repo.AddQuestion(ctx, newTestDefenseTurn("attempt-2", 1)))

	// When listing one
	got, err := repo.ListByAttempt(ctx, "attempt-1")

	// Then only its own turn comes back
	require.NoError(t, err)
	assert.Equal(t, []exercise.DefenseTurn{newTestDefenseTurn("attempt-1", 1)}, got)
}

func TestDefenseRepository_AddQuestion_refusesAPositionThatWasAlreadyAsked(t *testing.T) {
	// Given a stored first question
	repo, _ := newTestDefenseRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.AddQuestion(ctx, newTestDefenseTurn("attempt-1", 1)))
	again := newTestDefenseTurn("attempt-1", 1)
	again.ID = "another-id"

	// When storing another question at the same position
	err := repo.AddQuestion(ctx, again)

	// Then the database refuses it and keeps one turn
	assert.Error(t, err)
	got, listErr := repo.ListByAttempt(ctx, "attempt-1")
	require.NoError(t, listErr)
	assert.Len(t, got, 1)
}

func TestDefenseRepository_AddQuestion_concurrentCallsStoreOnlyOne(t *testing.T) {
	// Given several calls adding the same position at once
	repo, _ := newTestDefenseRepository(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			turn := newTestDefenseTurn("attempt-1", 1)
			turn.ID = fmt.Sprintf("turn-%d", i)
			_ = repo.AddQuestion(ctx, turn)
		}()
	}

	// When they finish
	wg.Wait()

	// Then exactly one question exists
	got, err := repo.ListByAttempt(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestDefenseRepository_SaveAnswer_storesTheAnswerAndItsTime(t *testing.T) {
	// Given an asked question
	repo, _ := newTestDefenseRepository(t)
	ctx := context.Background()
	turn := newTestDefenseTurn("attempt-1", 1)
	require.NoError(t, repo.AddQuestion(ctx, turn))
	answeredAt := defenseTestTime.Add(time.Minute)

	// When saving its answer
	require.NoError(t, repo.SaveAnswer(ctx, turn.ID, "Because \"3\" is odd — ünïcode", answeredAt))

	// Then the turn carries the answer and when it was given
	got, err := repo.ListByAttempt(ctx, "attempt-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Because \"3\" is odd — ünïcode", got[0].Answer)
	assert.Equal(t, answeredAt, got[0].AnsweredAt)
	assert.True(t, got[0].Answered())
}

func TestDefenseRepository_SaveAnswer_refusesASecondAnswerAndAnUnknownTurn(t *testing.T) {
	// Given an answered question
	repo, _ := newTestDefenseRepository(t)
	ctx := context.Background()
	turn := newTestDefenseTurn("attempt-1", 1)
	require.NoError(t, repo.AddQuestion(ctx, turn))
	require.NoError(t, repo.SaveAnswer(ctx, turn.ID, "first", defenseTestTime))

	// When answering it again, or an unknown turn
	again := repo.SaveAnswer(ctx, turn.ID, "second", defenseTestTime.Add(time.Hour))
	unknown := repo.SaveAnswer(ctx, "missing", "x", defenseTestTime)

	// Then both are refused and the first answer stays
	assert.ErrorIs(t, again, exercise.ErrNoQuestionPending)
	assert.ErrorIs(t, unknown, exercise.ErrNoQuestionPending)
	got, err := repo.ListByAttempt(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, "first", got[0].Answer)
}
