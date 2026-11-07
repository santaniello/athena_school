package exercise

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type hintFixture struct {
	service    *HintService
	challenges *domainmocks.MockChallengeRepository
	rubrics    *domainmocks.MockRubricRepository
	attempts   *domainmocks.MockAttemptRepository
	hints      *domainmocks.MockHintRepository
	generator  *domainmocks.MockHintGenerator
	clock      *fakeClock
}

func newHintFixture(t *testing.T) *hintFixture {
	t.Helper()
	f := &hintFixture{
		challenges: domainmocks.NewMockChallengeRepository(t),
		rubrics:    domainmocks.NewMockRubricRepository(t),
		attempts:   domainmocks.NewMockAttemptRepository(t),
		hints:      domainmocks.NewMockHintRepository(t),
		generator:  domainmocks.NewMockHintGenerator(t),
		clock:      &fakeClock{now: fixedNow},
	}
	f.service = NewHintService(f.challenges, f.rubrics, f.attempts, f.hints, f.generator, f.clock)
	f.service.newID = func() string { return "hint-id" }
	return f
}

func hintAttempt() domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-2",
		State: domainexercise.AttemptInProgress, StartedAt: fixedNow.Add(-10 * time.Minute), Deadline: fixedNow.Add(35 * time.Minute),
	}
}

func hintsGiven(count int) []domainexercise.Hint {
	hints := make([]domainexercise.Hint, 0, count)
	for position := 1; position <= count; position++ {
		hints = append(hints, domainexercise.Hint{ID: "h", AttemptID: "attempt-1", Position: position, Content: "earlier hint"})
	}
	return hints
}

// expectInputs makes the fixture return the running attempt, the given hints,
// the challenge and the rubric.
func (f *hintFixture) expectInputs(ctx context.Context, previous []domainexercise.Hint) {
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(previous, nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(attemptRubric(), nil)
}

func TestHintService_RequestHint_generatesAndStoresTheNextHint(t *testing.T) {
	// Given a running attempt with one hint already given
	f := newHintFixture(t)
	ctx := context.Background()
	previous := hintsGiven(1)
	f.expectInputs(ctx, previous)
	f.generator.EXPECT().GenerateHint(ctx, domainexercise.HintInput{
		Challenge: attemptChallenge(), Rubric: attemptRubric(), BoardXML: "<mxfile/>", Previous: previous,
	}).Return("  What happens if the cache is down?\n", nil)
	expected := domainexercise.Hint{
		ID: "hint-id", AttemptID: "attempt-1", Position: 2, Content: "What happens if the cache is down?", CreatedAt: fixedNow,
	}
	f.hints.EXPECT().Add(ctx, expected).Return(nil)

	// When asking for a hint with the current board
	hint, err := f.service.RequestHint(ctx, "attempt-1", "<mxfile/>")

	// Then the trimmed hint is stored at the next position and returned
	require.NoError(t, err)
	assert.Equal(t, expected, hint)
}

func TestHintService_RequestHint_worksWithAnEmptyBoard(t *testing.T) {
	// Given a running attempt and no board content yet
	f := newHintFixture(t)
	ctx := context.Background()
	f.expectInputs(ctx, []domainexercise.Hint{})
	f.generator.EXPECT().GenerateHint(ctx, domainexercise.HintInput{
		Challenge: attemptChallenge(), Rubric: attemptRubric(), BoardXML: "", Previous: []domainexercise.Hint{},
	}).Return("Where would you start?", nil)
	f.hints.EXPECT().Add(ctx, mock.MatchedBy(func(h domainexercise.Hint) bool { return h.Position == 1 })).Return(nil)

	// When asking for the first hint
	hint, err := f.service.RequestHint(ctx, "attempt-1", "")

	// Then it is the first one
	require.NoError(t, err)
	assert.Equal(t, 1, hint.Position)
}

func TestHintService_RequestHint_acceptsABoardAtTheSizeLimit(t *testing.T) {
	// Given a board of exactly the submission limit
	f := newHintFixture(t)
	ctx := context.Background()
	board := strings.Repeat("x", domainexercise.MaxSubmissionContentBytes)
	f.expectInputs(ctx, []domainexercise.Hint{})
	f.generator.EXPECT().GenerateHint(ctx, domainexercise.HintInput{
		Challenge: attemptChallenge(), Rubric: attemptRubric(), BoardXML: board, Previous: []domainexercise.Hint{},
	}).Return("A hint", nil)
	f.hints.EXPECT().Add(ctx, mock.MatchedBy(func(domainexercise.Hint) bool { return true })).Return(nil)

	// When asking for a hint
	_, err := f.service.RequestHint(ctx, "attempt-1", board)

	// Then it is accepted
	require.NoError(t, err)
}

func TestHintService_RequestHint_refusesABoardOverTheSizeLimit(t *testing.T) {
	// Given a board one byte over the limit
	f := newHintFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.Hint{}, nil)

	// When asking for a hint
	_, err := f.service.RequestHint(ctx, "attempt-1", strings.Repeat("x", domainexercise.MaxSubmissionContentBytes+1))

	// Then it is refused without calling the model
	assert.ErrorIs(t, err, domainexercise.ErrSubmissionTooLarge)
}

func TestHintService_RequestHint_unknownAttempt(t *testing.T) {
	// Given no such attempt
	f := newHintFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When asking for a hint
	_, err := f.service.RequestHint(ctx, "attempt-1", "")

	// Then it is not found
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
}

func TestHintService_RequestHint_refusesAnAttemptThatIsNotInProgress(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{
		domainexercise.AttemptSubmitted, domainexercise.AttemptEvaluating, domainexercise.AttemptEvaluated,
		domainexercise.AttemptEvaluationFailed, domainexercise.AttemptFinished,
	} {
		t.Run(string(state), func(t *testing.T) {
			// Given an attempt that is past the exam
			f := newHintFixture(t)
			ctx := context.Background()
			attempt := hintAttempt()
			attempt.State = state
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attempt, nil)

			// When asking for a hint
			_, err := f.service.RequestHint(ctx, "attempt-1", "")

			// Then it is refused and no model is called
			assert.ErrorIs(t, err, domainexercise.ErrAttemptNotInProgress)
		})
	}
}

func TestHintService_RequestHint_refusesAtAndAfterTheDeadline(t *testing.T) {
	for name, offset := range map[string]time.Duration{"at the deadline": 0, "after the deadline": time.Second} {
		t.Run(name, func(t *testing.T) {
			// Given a running attempt whose deadline has been reached
			f := newHintFixture(t)
			ctx := context.Background()
			attempt := hintAttempt()
			attempt.Deadline = fixedNow.Add(-offset)
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attempt, nil)

			// When asking for a hint
			_, err := f.service.RequestHint(ctx, "attempt-1", "")

			// Then it is refused
			assert.ErrorIs(t, err, domainexercise.ErrAttemptNotInProgress)
		})
	}
}

func TestHintService_RequestHint_refusesAFourthHintWithoutCallingTheModel(t *testing.T) {
	// Given an attempt that already received MaxHints hints
	f := newHintFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(hintsGiven(domainexercise.MaxHints), nil)

	// When asking for another
	_, err := f.service.RequestHint(ctx, "attempt-1", "")

	// Then no hints are left and the generator was not called
	assert.ErrorIs(t, err, domainexercise.ErrNoHintsLeft)
}

func TestHintService_RequestHint_aProviderErrorStoresNothingAndCanBeRepeated(t *testing.T) {
	// Given a generator that fails once and then works
	f := newHintFixture(t)
	ctx := context.Background()
	providerErr := errors.New("provider down")
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.Hint{}, nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(attemptRubric(), nil)
	f.generator.EXPECT().GenerateHint(ctx, mock.MatchedBy(func(domainexercise.HintInput) bool { return true })).Return("", providerErr).Once()
	f.generator.EXPECT().GenerateHint(ctx, mock.MatchedBy(func(domainexercise.HintInput) bool { return true })).Return("Try again", nil).Once()
	f.hints.EXPECT().Add(ctx, mock.MatchedBy(func(h domainexercise.Hint) bool { return h.Position == 1 })).Return(nil).Once()

	// When the first call fails and the second works
	_, firstErr := f.service.RequestHint(ctx, "attempt-1", "")
	hint, secondErr := f.service.RequestHint(ctx, "attempt-1", "")

	// Then nothing was stored for the failure and the retry takes position 1
	assert.ErrorIs(t, firstErr, providerErr)
	require.NoError(t, secondErr)
	assert.Equal(t, 1, hint.Position)
}

func TestHintService_RequestHint_aCancelledCallStoresNothing(t *testing.T) {
	// Given a generator that stops because the call was cancelled
	f := newHintFixture(t)
	ctx := context.Background()
	f.expectInputs(ctx, []domainexercise.Hint{})
	f.generator.EXPECT().GenerateHint(ctx, mock.MatchedBy(func(domainexercise.HintInput) bool { return true })).Return("", context.Canceled)

	// When requesting a hint
	_, err := f.service.RequestHint(ctx, "attempt-1", "")

	// Then the cancellation is returned and no hint was added
	assert.ErrorIs(t, err, context.Canceled)
}

func TestHintService_RequestHint_failedReadsOfTheInputsAreReturned(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		arrange func(f *hintFixture, ctx context.Context)
	}{
		{"attempt", func(f *hintFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, boom)
		}},
		{"hints", func(f *hintFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
			f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, boom)
		}},
		{"challenge", func(f *hintFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
			f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.Hint{}, nil)
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{}, boom)
		}},
		{"rubric", func(f *hintFixture, ctx context.Context) {
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
			f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.Hint{}, nil)
			f.challenges.EXPECT().Get(ctx, "challenge-1").Return(attemptChallenge(), nil)
			f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(domainexercise.RubricVersion{}, boom)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a store that fails reading the input
			f := newHintFixture(t)
			ctx := context.Background()
			tt.arrange(f, ctx)

			// When requesting a hint
			_, err := f.service.RequestHint(ctx, "attempt-1", "")

			// Then the failure is returned and the model is not called
			assert.ErrorIs(t, err, boom)
		})
	}
}

func TestHintService_RequestHint_aStoreFailureOrRefusalIsReturned(t *testing.T) {
	for name, storeErr := range map[string]error{
		"database error":           errors.New("disk full"),
		"cap reached concurrently": domainexercise.ErrNoHintsLeft,
		"position taken meanwhile": domainexercise.ErrHintInProgress,
	} {
		t.Run(name, func(t *testing.T) {
			// Given a repository that refuses the generated hint
			f := newHintFixture(t)
			ctx := context.Background()
			f.expectInputs(ctx, []domainexercise.Hint{})
			f.generator.EXPECT().GenerateHint(ctx, mock.MatchedBy(func(domainexercise.HintInput) bool { return true })).Return("A hint", nil)
			f.hints.EXPECT().Add(ctx, mock.MatchedBy(func(domainexercise.Hint) bool { return true })).Return(storeErr)

			// When requesting a hint
			_, err := f.service.RequestHint(ctx, "attempt-1", "")

			// Then the cause reaches the caller
			assert.ErrorIs(t, err, storeErr)
		})
	}
}

func TestHintService_RequestHint_aSecondRequestWhileOneRunsIsRefused(t *testing.T) {
	// Given a first request blocked inside the generator
	f := newHintFixture(t)
	ctx := context.Background()
	f.expectInputs(ctx, []domainexercise.Hint{})
	entered, release := make(chan struct{}), make(chan struct{})
	f.generator.EXPECT().GenerateHint(ctx, mock.MatchedBy(func(domainexercise.HintInput) bool { return true })).
		RunAndReturn(func(context.Context, domainexercise.HintInput) (string, error) {
			close(entered)
			<-release
			return "A hint", nil
		})
	f.hints.EXPECT().Add(ctx, mock.MatchedBy(func(domainexercise.Hint) bool { return true })).Return(nil)
	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		_, firstErr = f.service.RequestHint(ctx, "attempt-1", "")
	}()
	<-entered

	// When a second request arrives for the same attempt
	_, err := f.service.RequestHint(ctx, "attempt-1", "")
	close(release)
	wg.Wait()

	// Then it is refused while the first one completes
	assert.ErrorIs(t, err, domainexercise.ErrHintInProgress)
	require.NoError(t, firstErr)
}

func TestHintService_RequestHint_releasesTheGuardAfterAFailure(t *testing.T) {
	// Given a request that failed
	f := newHintFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound).Twice()

	// When requesting again
	_, first := f.service.RequestHint(ctx, "attempt-1", "")
	_, second := f.service.RequestHint(ctx, "attempt-1", "")

	// Then the second is not mistaken for a concurrent request
	assert.ErrorIs(t, first, domainexercise.ErrAttemptNotFound)
	assert.ErrorIs(t, second, domainexercise.ErrAttemptNotFound)
}

func TestHintService_RequestHint_attemptsDoNotBlockEachOther(t *testing.T) {
	// Given a request running for attempt-1
	f := newHintFixture(t)
	ctx := context.Background()
	f.service.running["attempt-1"] = struct{}{}
	f.attempts.EXPECT().Get(ctx, "attempt-2").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When requesting a hint for attempt-2
	_, err := f.service.RequestHint(ctx, "attempt-2", "")

	// Then it proceeds on its own
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
}

func TestHintService_ListHints_returnsTheAttemptsHints(t *testing.T) {
	// Given an attempt with two hints
	f := newHintFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(hintsGiven(2), nil)

	// When listing them
	got, err := f.service.ListHints(ctx, "attempt-1")

	// Then they come back in order
	require.NoError(t, err)
	assert.Equal(t, hintsGiven(2), got)
}

func TestHintService_ListHints_keepsHintsOfAFinishedAttempt(t *testing.T) {
	// Given a submitted attempt
	f := newHintFixture(t)
	ctx := context.Background()
	attempt := hintAttempt()
	attempt.State = domainexercise.AttemptSubmitted
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(attempt, nil)
	f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(hintsGiven(1), nil)

	// When listing its hints
	got, err := f.service.ListHints(ctx, "attempt-1")

	// Then they are still readable
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestHintService_ListHints_failures(t *testing.T) {
	boom := errors.New("boom")
	t.Run("unknown attempt", func(t *testing.T) {
		// Given no such attempt
		f := newHintFixture(t)
		ctx := context.Background()
		f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

		// When listing
		_, err := f.service.ListHints(ctx, "attempt-1")

		// Then it is not found
		assert.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
	})
	t.Run("store error", func(t *testing.T) {
		// Given a failing hint store
		f := newHintFixture(t)
		ctx := context.Background()
		f.attempts.EXPECT().Get(ctx, "attempt-1").Return(hintAttempt(), nil)
		f.hints.EXPECT().ListByAttempt(ctx, "attempt-1").Return(nil, boom)

		// When listing
		_, err := f.service.ListHints(ctx, "attempt-1")

		// Then the error is returned
		assert.ErrorIs(t, err, boom)
	})
}
