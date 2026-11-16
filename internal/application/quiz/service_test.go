package quiz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
	domainmocks "github.com/santaniello/athena/internal/domain/quiz/mocks"
)

func storedQuiz() domainquiz.Quiz {
	question := func(id string, position int) domainquiz.Question {
		return domainquiz.Question{
			ID: id, Position: position, Kind: domainquiz.KindMultipleChoice, Prompt: "p",
			Options: []string{"A", "B"}, CorrectIndex: 1, Explanation: "e",
		}
	}
	return domainquiz.Quiz{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: domainquiz.DifficultyEasy,
		Questions: []domainquiz.Question{question("q1", 1), question("q2", 2), question("q3", 3)}, CreatedAt: fixedNow,
	}
}

func idx(i int) *int { return &i }

func newService(t *testing.T) (*Service, *domainmocks.MockRepository) {
	t.Helper()
	repo := domainmocks.NewMockRepository(t)
	service := NewService(repo)
	service.now = func() time.Time { return fixedNow }
	service.newID = func() string { return "attempt-1" }
	return service, repo
}

func TestService_SubmitAttempt_gradesAndStoresTheAttempt(t *testing.T) {
	// Given a stored quiz and a user who got 2 of 3 right
	service, repo := newService(t)
	ctx := context.Background()
	repo.EXPECT().GetQuiz(ctx, "quiz-1").Return(storedQuiz(), nil).Once()
	want := domainquiz.Attempt{
		ID: "attempt-1", QuizID: "quiz-1", CorrectCount: 2, Total: 3, Score: 6.7, FinishedAt: fixedNow,
		Answers: []domainquiz.Answer{
			{QuestionID: "q1", SelectedIndex: idx(1), Correct: true},
			{QuestionID: "q2", SelectedIndex: idx(1), Correct: true},
			{QuestionID: "q3", SelectedIndex: idx(0), Correct: false},
		},
	}
	repo.EXPECT().CreateAttempt(ctx, want).Return(nil).Once()

	// When the attempt is submitted
	got, err := service.SubmitAttempt(ctx, "quiz-1", []domainquiz.Selection{
		{QuestionID: "q1", SelectedIndex: idx(1)}, {QuestionID: "q2", SelectedIndex: idx(1)}, {QuestionID: "q3", SelectedIndex: idx(0)},
	})

	// Then the graded attempt is stored with an ID and the finish time, and returned
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestService_SubmitAttempt_unknownQuiz(t *testing.T) {
	service, repo := newService(t)
	repo.EXPECT().GetQuiz(context.Background(), "nope").Return(domainquiz.Quiz{}, domainquiz.ErrQuizNotFound).Once()

	_, err := service.SubmitAttempt(context.Background(), "nope", nil)

	assert.ErrorIs(t, err, domainquiz.ErrQuizNotFound)
}

func TestService_SubmitAttempt_badSelectionStoresNothing(t *testing.T) {
	// Given a selection naming a question the quiz lacks (the strict mock fails on a write)
	service, repo := newService(t)
	repo.EXPECT().GetQuiz(context.Background(), "quiz-1").Return(storedQuiz(), nil).Once()

	_, err := service.SubmitAttempt(context.Background(), "quiz-1", []domainquiz.Selection{{QuestionID: "zzz", SelectedIndex: idx(0)}})

	assert.ErrorIs(t, err, domainquiz.ErrSelectionUnknownQuestion)
}

func TestService_SubmitAttempt_repositoryFailureIsReturned(t *testing.T) {
	service, repo := newService(t)
	ctx := context.Background()
	repo.EXPECT().GetQuiz(ctx, "quiz-1").Return(storedQuiz(), nil).Once()
	boom := errors.New("disk full")
	blank := domainquiz.Attempt{
		ID: "attempt-1", QuizID: "quiz-1", Total: 3, FinishedAt: fixedNow,
		Answers: []domainquiz.Answer{{QuestionID: "q1"}, {QuestionID: "q2"}, {QuestionID: "q3"}},
	}
	repo.EXPECT().CreateAttempt(ctx, blank).Return(boom).Once()

	_, err := service.SubmitAttempt(ctx, "quiz-1", nil)

	assert.ErrorIs(t, err, boom)
}

func TestService_GetAttemptResult_joinsTheAttemptWithItsQuiz(t *testing.T) {
	// Given a stored attempt and its quiz
	service, repo := newService(t)
	ctx := context.Background()
	attempt := domainquiz.Attempt{ID: "attempt-1", QuizID: "quiz-1", Score: 10}
	repo.EXPECT().GetAttempt(ctx, "attempt-1").Return(attempt, nil).Once()
	repo.EXPECT().GetQuiz(ctx, "quiz-1").Return(storedQuiz(), nil).Once()

	// When the result is read
	result, err := service.GetAttemptResult(ctx, "attempt-1")

	// Then it carries both, so the answer key and explanations can be shown
	require.NoError(t, err)
	assert.Equal(t, AttemptResult{Quiz: storedQuiz(), Attempt: attempt}, result)
}

func TestService_GetAttemptResult_failures(t *testing.T) {
	ctx := context.Background()
	t.Run("unknown attempt", func(t *testing.T) {
		service, repo := newService(t)
		repo.EXPECT().GetAttempt(ctx, "x").Return(domainquiz.Attempt{}, domainquiz.ErrAttemptNotFound).Once()
		_, err := service.GetAttemptResult(ctx, "x")
		assert.ErrorIs(t, err, domainquiz.ErrAttemptNotFound)
	})
	t.Run("quiz gone", func(t *testing.T) {
		service, repo := newService(t)
		repo.EXPECT().GetAttempt(ctx, "a").Return(domainquiz.Attempt{QuizID: "q"}, nil).Once()
		repo.EXPECT().GetQuiz(ctx, "q").Return(domainquiz.Quiz{}, domainquiz.ErrQuizNotFound).Once()
		_, err := service.GetAttemptResult(ctx, "a")
		assert.ErrorIs(t, err, domainquiz.ErrQuizNotFound)
	})
}

func TestService_readsAndDeletesPassThroughTheRepository(t *testing.T) {
	ctx := context.Background()
	service, repo := newService(t)
	summaries := []domainquiz.Summary{{ID: "quiz-1", AttemptCount: 2}}
	attempts := []domainquiz.Attempt{{ID: "attempt-1"}}
	repo.EXPECT().ListQuizzes(ctx).Return(summaries, nil).Once()
	repo.EXPECT().GetQuiz(ctx, "quiz-1").Return(storedQuiz(), nil).Once()
	repo.EXPECT().ListAttempts(ctx, "quiz-1").Return(attempts, nil).Once()
	repo.EXPECT().DeleteQuiz(ctx, "quiz-1").Return(nil).Once()

	gotSummaries, err := service.ListQuizzes(ctx)
	require.NoError(t, err)
	assert.Equal(t, summaries, gotSummaries)

	gotQuiz, err := service.GetQuiz(ctx, "quiz-1")
	require.NoError(t, err)
	assert.Equal(t, storedQuiz(), gotQuiz)

	gotAttempts, err := service.ListAttempts(ctx, "quiz-1")
	require.NoError(t, err)
	assert.Equal(t, attempts, gotAttempts)

	assert.NoError(t, service.DeleteQuiz(ctx, "quiz-1"))
}

func TestService_readFailuresAreWrappedAndKeepTheCause(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	service, repo := newService(t)
	repo.EXPECT().ListQuizzes(ctx).Return(nil, boom).Once()
	repo.EXPECT().GetQuiz(ctx, "q").Return(domainquiz.Quiz{}, boom).Once()
	repo.EXPECT().ListAttempts(ctx, "q").Return(nil, boom).Once()
	repo.EXPECT().DeleteQuiz(ctx, "q").Return(boom).Once()

	_, err := service.ListQuizzes(ctx)
	assert.ErrorIs(t, err, boom)
	_, err = service.GetQuiz(ctx, "q")
	assert.ErrorIs(t, err, boom)
	_, err = service.ListAttempts(ctx, "q")
	assert.ErrorIs(t, err, boom)
	assert.ErrorIs(t, service.DeleteQuiz(ctx, "q"), boom)
}

func TestNewService_defaultsAreRealImplementations(t *testing.T) {
	// When a service is built without test overrides
	service := NewService(nil)

	// Then it stamps the current time and makes unique IDs
	assert.WithinDuration(t, time.Now(), service.now(), time.Minute)
	assert.NotEqual(t, service.newID(), service.newID())
}

func TestService_ListSessionQuizzes_passesThroughTheRepository(t *testing.T) {
	// Given a repository that knows the session's offer quizzes
	service, repo := newService(t)
	ctx := context.Background()
	want := []domainquiz.Summary{{ID: "quiz-1", SessionID: "session-1", MessageID: "message-1"}}
	repo.EXPECT().ListBySession(ctx, "session-1").Return(want, nil).Once()

	// When listing the session's quizzes
	got, err := service.ListSessionQuizzes(ctx, "session-1")

	// Then they come back as they are
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestService_ListSessionQuizzes_wrapsAFailureAndKeepsTheCause(t *testing.T) {
	// Given a failing repository
	service, repo := newService(t)
	boom := errors.New("db down")
	repo.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, boom).Once()

	// When listing the session's quizzes
	_, err := service.ListSessionQuizzes(context.Background(), "session-1")

	// Then the cause is kept
	require.ErrorIs(t, err, boom)
}
