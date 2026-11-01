package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

var exerciseNow = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

type bindingClock struct{ now time.Time }

func (c bindingClock) Now() time.Time { return c.now }

type exerciseFixture struct {
	app         *App
	challenges  *domainmocks.MockChallengeRepository
	rubrics     *domainmocks.MockRubricRepository
	attempts    *domainmocks.MockAttemptRepository
	submissions *domainmocks.MockSubmissionRepository
	sessions    *exercisemocks.MockSessionExists
	tx          *exercisemocks.MockTransactor
}

func newExerciseFixture(t *testing.T) *exerciseFixture {
	t.Helper()
	f := &exerciseFixture{
		challenges:  domainmocks.NewMockChallengeRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		attempts:    domainmocks.NewMockAttemptRepository(t),
		submissions: domainmocks.NewMockSubmissionRepository(t),
		sessions:    exercisemocks.NewMockSessionExists(t),
		tx:          exercisemocks.NewMockTransactor(t),
	}
	service := applicationexercise.NewAttemptService(
		f.challenges, f.rubrics, f.attempts, f.submissions, domainmocks.NewMockSessionExerciseRepository(t),
		f.sessions, f.tx, bindingClock{now: exerciseNow},
	)
	f.app = NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithAttemptService(service))
	f.app.ctx = context.Background()
	return f
}

// codeOf returns the ExerciseError code carried by err, failing the test if
// err is not an ExerciseError whose text is its JSON form.
func codeOf(t *testing.T, err error) ExerciseError {
	t.Helper()
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	var decoded ExerciseError
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &decoded))
	assert.Equal(t, *coded, decoded)
	return decoded
}

func storedBindingAttempt() domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-1", SessionExerciseID: "link-1",
		State: domainexercise.AttemptInProgress, StartedAt: exerciseNow, Deadline: exerciseNow.Add(45 * time.Minute),
	}
}

func (f *exerciseFixture) expectView(ctx context.Context) {
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{
		ID: "challenge-1", Title: "Design a URL shortener", Topic: "caching",
	}, nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-1").Return(domainexercise.RubricVersion{
		ID: "rubric-1", Criteria: []domainexercise.RubricCriterion{{
			Key: "architecture", Name: "Overall architecture", Description: "Coherent.", Weight: 100, Mandatory: true,
			Levels: [domainexercise.LevelCount]string{"0", "1", "2", "3", "4"},
		}},
	}, nil)
}

func TestApp_GetAttempt_mapsAnInProgressAttemptToItsDTO(t *testing.T) {
	// Given an attempt in progress
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedBindingAttempt(), nil)
	f.expectView(ctx)

	// When the frontend reads it
	dto, err := f.app.GetAttempt("attempt-1")

	// Then the DTO carries the attempt, the challenge details and the criteria, with no submission
	require.NoError(t, err)
	assert.Equal(t, AttemptDTO{
		ID: "attempt-1", ChallengeID: "challenge-1", ChallengeTitle: "Design a URL shortener", ChallengeTopic: "caching",
		SessionExerciseID: "link-1", State: "IN_PROGRESS",
		StartedAt: "2026-11-01T10:00:00Z", Deadline: "2026-11-01T10:45:00Z",
		Criteria: []CriterionDTO{{
			Key: "architecture", Name: "Overall architecture", Description: "Coherent.", Weight: 100, Mandatory: true,
			Levels: []string{"0", "1", "2", "3", "4"},
		}},
	}, dto)
}

func TestApp_GetAttempt_mapsASubmittedAttemptWithItsSubmission(t *testing.T) {
	// Given a submitted attempt
	f := newExerciseFixture(t)
	ctx := context.Background()
	submitted := storedBindingAttempt()
	submitted.State = domainexercise.AttemptSubmitted
	submitted.SubmittedAt = exerciseNow.Add(10 * time.Minute)
	submitted.SubmitReason = domainexercise.SubmitReasonUser
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(submitted, nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(domainexercise.Submission{
		ContentType: "application/drawio", Content: "<mxfile/>", Checksum: "abc", SubmittedAt: submitted.SubmittedAt,
	}, nil)
	f.expectView(ctx)

	// When the frontend reads it
	dto, err := f.app.GetAttempt("attempt-1")

	// Then the submission and its reason are exposed
	require.NoError(t, err)
	assert.Equal(t, "SUBMITTED", dto.State)
	assert.Equal(t, "USER", dto.SubmitReason)
	assert.Equal(t, "2026-11-01T10:10:00Z", dto.SubmittedAt)
	assert.Equal(t, &SubmissionDTO{
		ContentType: "application/drawio", Content: "<mxfile/>", Checksum: "abc", SubmittedAt: "2026-11-01T10:10:00Z",
	}, dto.Submission)
}

func TestApp_StartAttempt_startsAStandaloneAttemptAndReturnsIt(t *testing.T) {
	// Given a startable challenge and no attempt running
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(domainexercise.Challenge{
		ID: "challenge-1", Title: "T", Topic: "caching", TimeLimitMinutes: 30,
	}, nil)
	f.rubrics.EXPECT().Latest(ctx, "challenge-1").Return(domainexercise.RubricVersion{ID: "rubric-1", ChallengeID: "challenge-1"}, nil)
	f.attempts.EXPECT().FindInProgress(ctx).Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)
	f.tx.EXPECT().WithinTx(ctx, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	f.attempts.EXPECT().Create(ctx, mock.MatchedBy(func(attempt domainexercise.ChallengeAttempt) bool {
		return attempt.ChallengeID == "challenge-1" && attempt.SessionExerciseID == "" &&
			attempt.Deadline.Equal(exerciseNow.Add(30*time.Minute))
	})).Return(nil)

	// When the frontend starts it with a blank session id
	dto, err := f.app.StartAttempt("challenge-1", "  ", false)

	// Then a standalone attempt in progress is returned
	require.NoError(t, err)
	assert.Equal(t, "IN_PROGRESS", dto.State)
	assert.Equal(t, "2026-11-01T10:30:00Z", dto.Deadline)
	assert.Empty(t, dto.SessionExerciseID)
}

func TestApp_SubmitAttempt_returnsTheSubmittedAttempt(t *testing.T) {
	// Given an attempt in progress and a clock before its deadline
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedBindingAttempt(), nil)
	f.attempts.EXPECT().Submit(ctx, mock.MatchedBy(func(a domainexercise.ChallengeAttempt) bool {
		return a.State == domainexercise.AttemptSubmitted && a.SubmitReason == domainexercise.SubmitReasonUser
	}), mock.MatchedBy(func(s domainexercise.Submission) bool {
		return s.AttemptID == "attempt-1" && s.Content == "<mxfile/>"
	})).Return(nil)
	f.expectView(ctx)

	// When the frontend submits
	dto, err := f.app.SubmitAttempt("attempt-1", "application/drawio", "<mxfile/>")

	// Then the attempt comes back submitted with its submission
	require.NoError(t, err)
	assert.Equal(t, "SUBMITTED", dto.State)
	require.NotNil(t, dto.Submission)
	assert.Equal(t, "<mxfile/>", dto.Submission.Content)
}

func TestApp_DiscardAttempt_deletesTheAttempt(t *testing.T) {
	// Given an unfinished attempt
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(storedBindingAttempt(), nil)
	f.attempts.EXPECT().DeleteUnfinished(ctx, "attempt-1").Return(nil)

	// When the frontend discards it
	err := f.app.DiscardAttempt("attempt-1")

	// Then it succeeds
	assert.NoError(t, err)
}

func TestApp_ListAttempts_returnsADTOPerAttemptAndAnEmptyListWhenNone(t *testing.T) {
	// Given a challenge with one running attempt
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-1").Return([]domainexercise.ChallengeAttempt{storedBindingAttempt()}, nil)
	f.expectView(ctx)
	f.attempts.EXPECT().ListByChallenge(ctx, "challenge-2").Return(nil, nil)

	// When the frontend lists both challenges
	dtos, err := f.app.ListAttempts("challenge-1")
	none, noneErr := f.app.ListAttempts("challenge-2")

	// Then it gets one DTO, and an empty (non-nil) list
	require.NoError(t, err)
	require.Len(t, dtos, 1)
	assert.Equal(t, "attempt-1", dtos[0].ID)
	require.NoError(t, noneErr)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func TestApp_exerciseErrors_mapEverySentinelToItsCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"attempt in progress", &applicationexercise.AttemptInProgressError{AttemptID: "running-1"}, exerciseCodeAttemptInProgress},
		{"challenge not found", domainexercise.ErrChallengeNotFound, exerciseCodeChallengeNotFound},
		{"no rubric", applicationexercise.ErrChallengeHasNoRubric, exerciseCodeChallengeHasNoRubric},
		{"unknown session", applicationexercise.ErrSourceSessionNotFound, exerciseCodeSessionNotFound},
		{"session not linked", domainexercise.ErrSessionExerciseNotFound, exerciseCodeSessionNotLinked},
		{"attempt not found", domainexercise.ErrAttemptNotFound, exerciseCodeAttemptNotFound},
		{"attempt not in progress", domainexercise.ErrAttemptNotInProgress, exerciseCodeAttemptNotInProgress},
		{"attempt finished", applicationexercise.ErrAttemptFinished, exerciseCodeAttemptFinished},
		{"submission too late", applicationexercise.ErrSubmissionTooLate, exerciseCodeSubmissionTooLate},
		{"content required", domainexercise.ErrSubmissionContentRequired, exerciseCodeSubmissionInvalid},
		{"content type required", domainexercise.ErrSubmissionContentTypeRequired, exerciseCodeSubmissionInvalid},
		{"content too large", domainexercise.ErrSubmissionTooLarge, exerciseCodeSubmissionInvalid},
		{"anything else", errors.New("disk failure: secret path"), exerciseCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a use case error, wrapped as the services do
			wrapped := fmt.Errorf("exercise: something: %w", tt.err)

			// When it reaches the binding boundary
			got := toExerciseError(wrapped)

			// Then it carries the stable code
			assert.Equal(t, tt.want, got.Code)
		})
	}
}

func TestApp_exerciseErrors_inProgressNamesTheRunningAttemptAndInternalHidesTheDetail(t *testing.T) {
	// Given an in-progress conflict and an unknown failure
	conflict := toExerciseError(&applicationexercise.AttemptInProgressError{AttemptID: "running-1"})
	internal := toExerciseError(errors.New("disk failure: secret path"))

	// When read as the frontend sees them (JSON text)
	decodedConflict := codeOf(t, conflict)
	decodedInternal := codeOf(t, internal)

	// Then the conflict names the running attempt and the internal error leaks nothing
	assert.Equal(t, "running-1", decodedConflict.AttemptID)
	assert.NotContains(t, decodedInternal.Message, "secret")
	assert.Empty(t, decodedInternal.AttemptID)
}

func TestApp_bindings_returnTheCodedErrorFromTheService(t *testing.T) {
	// Given an unknown attempt
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "ghost").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When the frontend reads it
	_, err := f.app.GetAttempt("ghost")

	// Then it gets the attempt_not_found code
	assert.Equal(t, exerciseCodeAttemptNotFound, codeOf(t, err).Code)
}

func TestApp_bindings_failuresFromEveryUseCaseAreCoded(t *testing.T) {
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.challenges.EXPECT().Get(ctx, "ghost").Return(domainexercise.Challenge{}, domainexercise.ErrChallengeNotFound)
	f.attempts.EXPECT().Get(ctx, "ghost").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound).Twice()
	f.attempts.EXPECT().ListByChallenge(ctx, "ghost").Return(nil, errors.New("boom"))
	calls := map[string]func() error{
		"StartAttempt":   func() error { _, err := f.app.StartAttempt("ghost", "", false); return err },
		"SubmitAttempt":  func() error { _, err := f.app.SubmitAttempt("ghost", "application/drawio", "x"); return err },
		"DiscardAttempt": func() error { return f.app.DiscardAttempt("ghost") },
		"ListAttempts":   func() error { _, err := f.app.ListAttempts("ghost"); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// Given a failing use case, When the frontend calls the binding
			err := call()

			// Then the error is an ExerciseError
			var coded *ExerciseError
			assert.ErrorAs(t, err, &coded)
		})
	}
}

func TestApp_Startup_discardsStaleAttemptsOnce(t *testing.T) {
	// Given attempts left behind by a crash
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().DeleteAllUnfinished(ctx).Return(2, nil).Once()

	// When the app starts
	f.app.Startup(ctx)

	// Then the cleanup ran exactly once (the strict mock verifies it)
	assert.Equal(t, ctx, f.app.ctx)
}

func TestApp_Startup_aFailedCleanupDoesNotStopStartup(t *testing.T) {
	// Given a cleanup that fails
	f := newExerciseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().DeleteAllUnfinished(ctx).Return(0, errors.New("disk failure"))

	// When the app starts
	f.app.Startup(ctx)

	// Then startup still completes and the context is stored
	assert.Equal(t, ctx, f.app.ctx)
}

func TestApp_bindings_rejectBlankIDsWithoutCallingTheService(t *testing.T) {
	f := newExerciseFixture(t)
	calls := map[string]func() error{
		"StartAttempt":   func() error { _, err := f.app.StartAttempt("  ", "", false); return err },
		"SubmitAttempt":  func() error { _, err := f.app.SubmitAttempt("", "application/drawio", "x"); return err },
		"DiscardAttempt": func() error { return f.app.DiscardAttempt(" ") },
		"GetAttempt":     func() error { _, err := f.app.GetAttempt(""); return err },
		"ListAttempts":   func() error { _, err := f.app.ListAttempts(""); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// Given a blank id, When calling the binding
			err := call()

			// Then it fails as invalid input (the strict mocks prove nothing else was called)
			assert.Equal(t, exerciseCodeInvalidInput, codeOf(t, err).Code)
		})
	}
}
