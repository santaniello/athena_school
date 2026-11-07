package desktop

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

type fakeHints struct {
	hint      domainexercise.Hint
	hints     []domainexercise.Hint
	err       error
	attemptID string
	boardXML  string
	listed    string
}

func (h *fakeHints) RequestHint(_ context.Context, attemptID, boardXML string) (domainexercise.Hint, error) {
	h.attemptID, h.boardXML = attemptID, boardXML
	return h.hint, h.err
}

func (h *fakeHints) ListHints(_ context.Context, attemptID string) ([]domainexercise.Hint, error) {
	h.listed = attemptID
	return h.hints, h.err
}

func newHintsApp(hints hintGiver) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithHintService(hints))
	app.ctx = context.Background()
	return app
}

var hintBindingTime = time.Date(2026, 11, 7, 9, 15, 0, 0, time.UTC)

func TestApp_RequestHint_returnsTheHint(t *testing.T) {
	// Given a service that delivers a hint
	hints := &fakeHints{hint: domainexercise.Hint{
		ID: "hint-1", AttemptID: "attempt-1", Position: 2, Content: "What if the cache is down?", CreatedAt: hintBindingTime,
	}}
	app := newHintsApp(hints)

	// When the frontend asks for a hint with the board
	got, err := app.RequestHint(" attempt-1 ", "<mxfile/>")

	// Then the DTO is returned and the use case got the trimmed id and the board untouched
	require.NoError(t, err)
	assert.Equal(t, HintDTO{
		ID: "hint-1", AttemptID: "attempt-1", Position: 2, Content: "What if the cache is down?", CreatedAt: "2026-11-07T09:15:00Z",
	}, got)
	assert.Equal(t, "attempt-1", hints.attemptID)
	assert.Equal(t, "<mxfile/>", hints.boardXML)
}

func TestApp_RequestHint_rejectsABlankAttemptId(t *testing.T) {
	// Given a service that must not be called
	hints := &fakeHints{}
	app := newHintsApp(hints)

	// When asking with a blank id
	_, err := app.RequestHint("  ", "")

	// Then it is invalid input
	assert.Equal(t, exerciseCodeInvalidInput, codeOf(t, err).Code)
	assert.Empty(t, hints.attemptID)
}

func TestApp_RequestHint_mapsErrorsToCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"no hints left", fmt.Errorf("wrapped: %w", domainexercise.ErrNoHintsLeft), exerciseCodeNoHintsLeft},
		{"request running", fmt.Errorf("wrapped: %w", domainexercise.ErrHintInProgress), exerciseCodeHintInProgress},
		{"attempt not in progress", domainexercise.ErrAttemptNotInProgress, exerciseCodeAttemptNotInProgress},
		{"unknown attempt", fmt.Errorf("wrapped: %w", domainexercise.ErrAttemptNotFound), exerciseCodeAttemptNotFound},
		{"board too large", domainexercise.ErrSubmissionTooLarge, exerciseCodeSubmissionTooLarge},
		{"cancelled", fmt.Errorf("wrapped: %w", context.Canceled), exerciseCodeCancelled},
		{"provider failure", errors.New("openrouter is down"), exerciseCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a service that fails
			app := newHintsApp(&fakeHints{err: tt.err})

			// When asking for a hint
			_, err := app.RequestHint("attempt-1", "")

			// Then the error carries the stable code
			assert.Equal(t, tt.code, codeOf(t, err).Code)
		})
	}
}

func TestApp_ListHints_returnsTheHintsInOrder(t *testing.T) {
	// Given two stored hints
	hints := &fakeHints{hints: []domainexercise.Hint{
		{ID: "h1", AttemptID: "attempt-1", Position: 1, Content: "first", CreatedAt: hintBindingTime},
		{ID: "h2", AttemptID: "attempt-1", Position: 2, Content: "second", CreatedAt: hintBindingTime},
	}}
	app := newHintsApp(hints)

	// When the frontend lists them
	got, err := app.ListHints(" attempt-1 ")

	// Then both come back as DTOs
	require.NoError(t, err)
	assert.Equal(t, []HintDTO{
		{ID: "h1", AttemptID: "attempt-1", Position: 1, Content: "first", CreatedAt: "2026-11-07T09:15:00Z"},
		{ID: "h2", AttemptID: "attempt-1", Position: 2, Content: "second", CreatedAt: "2026-11-07T09:15:00Z"},
	}, got)
	assert.Equal(t, "attempt-1", hints.listed)
}

func TestApp_ListHints_withoutHintsReturnsAnEmptyList(t *testing.T) {
	// Given an attempt without hints
	app := newHintsApp(&fakeHints{})

	// When listing them
	got, err := app.ListHints("attempt-1")

	// Then the list is empty and not nil, so the frontend gets []
	require.NoError(t, err)
	assert.Equal(t, []HintDTO{}, got)
}

func TestApp_ListHints_rejectsABlankIdAndMapsErrors(t *testing.T) {
	// Given a service that fails with an unknown attempt
	app := newHintsApp(&fakeHints{err: domainexercise.ErrAttemptNotFound})

	// When listing with a blank id and with an unknown one
	_, blank := app.ListHints(" ")
	_, unknown := app.ListHints("attempt-9")

	// Then they are invalid input and attempt_not_found
	assert.Equal(t, exerciseCodeInvalidInput, codeOf(t, blank).Code)
	assert.Equal(t, exerciseCodeAttemptNotFound, codeOf(t, unknown).Code)
}
