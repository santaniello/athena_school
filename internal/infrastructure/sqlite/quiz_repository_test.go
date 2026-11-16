package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/quiz"
)

var quizEpoch = time.Date(2026, 11, 16, 9, 0, 0, 123456789, time.UTC)

func newTestQuizRepository(t *testing.T) *QuizRepository {
	t.Helper()
	return NewQuizRepository(newTestDB(t))
}

func quizQuestion(id string, position int, kind quiz.QuestionKind, options ...string) quiz.Question {
	return quiz.Question{
		ID: id, Position: position, Kind: kind, Prompt: "Prompt " + id, Options: options,
		CorrectIndex: len(options) - 1, Explanation: "Because " + id,
	}
}

func newTestQuiz(id string, at time.Time) quiz.Quiz {
	return quiz.Quiz{
		ID: id, SessionID: "session-gone", SessionTitle: "Caching", Difficulty: quiz.DifficultyHard, CreatedAt: at,
		Questions: []quiz.Question{
			quizQuestion(id+"-q1", 1, quiz.KindMultipleChoice, "LRU", "FIFO", "LFU"),
			quizQuestion(id+"-q2", 2, quiz.KindTrueFalse, "True", "False"),
			quizQuestion(id+"-q3", 3, quiz.KindMultipleChoice, "A", "B"),
		},
	}
}

func pickIndex(i int) *int { return &i }

func newTestQuizAttempt(id, quizID string, score float64, at time.Time) quiz.Attempt {
	return quiz.Attempt{
		ID: id, QuizID: quizID, CorrectCount: 1, Total: 3, Score: score, FinishedAt: at,
		Answers: []quiz.Answer{
			{QuestionID: quizID + "-q1", SelectedIndex: pickIndex(2), Correct: true},
			{QuestionID: quizID + "-q2", SelectedIndex: pickIndex(0), Correct: false},
			{QuestionID: quizID + "-q3", SelectedIndex: nil, Correct: false},
		},
	}
}

func TestQuizRepository_CreateQuizThenGetQuiz_roundTripsEveryField(t *testing.T) {
	// Given a quiz whose session no longer exists (a quiz outlives its session)
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	original := newTestQuiz("quiz-1", quizEpoch)

	// When storing it and reading it back
	require.NoError(t, repo.CreateQuiz(ctx, original))
	got, err := repo.GetQuiz(ctx, "quiz-1")

	// Then nothing is lost: fields, question order, options and the nanosecond time
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestQuizRepository_GetQuiz_unknownIDIsNotFound(t *testing.T) {
	repo := newTestQuizRepository(t)

	_, err := repo.GetQuiz(context.Background(), "nope")

	assert.ErrorIs(t, err, quiz.ErrQuizNotFound)
}

func TestQuizRepository_CreateQuiz_isAllOrNothing(t *testing.T) {
	// Given a quiz whose last question repeats an earlier ID
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	broken := newTestQuiz("quiz-1", quizEpoch)
	broken.Questions[2].ID = broken.Questions[0].ID

	// When storing it
	err := repo.CreateQuiz(ctx, broken)

	// Then it fails and not even the quiz row is left behind
	require.Error(t, err)
	_, err = repo.GetQuiz(ctx, "quiz-1")
	assert.ErrorIs(t, err, quiz.ErrQuizNotFound)
}

func TestQuizRepository_CreateQuiz_joinsTheTransactionOnTheContext(t *testing.T) {
	// Given a quiz written inside a transaction that then fails
	db := newTestDB(t)
	repo := NewQuizRepository(db)
	boom := errors.New("boom")

	err := NewSQLTransactor(db).WithinTx(context.Background(), func(ctx context.Context) error {
		require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
		return boom
	})

	// Then the rollback takes the quiz with it
	require.ErrorIs(t, err, boom)
	_, err = repo.GetQuiz(context.Background(), "quiz-1")
	assert.ErrorIs(t, err, quiz.ErrQuizNotFound)
}

func TestQuizRepository_ListQuizzes_newestFirstWithAttemptSummary(t *testing.T) {
	// Given an old quiz with two attempts, a newer one with none
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	older, newer := newTestQuiz("quiz-old", quizEpoch), newTestQuiz("quiz-new", quizEpoch.Add(time.Hour))
	require.NoError(t, repo.CreateQuiz(ctx, older))
	require.NoError(t, repo.CreateQuiz(ctx, newer))
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a1", "quiz-old", 3.3, quizEpoch.Add(2*time.Hour))))
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a2", "quiz-old", 8.0, quizEpoch.Add(3*time.Hour))))

	// When listing
	got, err := repo.ListQuizzes(ctx)

	// Then the newest quiz is first, and the old one reports 2 attempts and the latest score
	require.NoError(t, err)
	latest := 8.0
	assert.Equal(t, []quiz.Summary{
		{ID: "quiz-new", SessionID: "session-gone", SessionTitle: "Caching", Difficulty: quiz.DifficultyHard, QuestionCount: 3, CreatedAt: newer.CreatedAt},
		{ID: "quiz-old", SessionID: "session-gone", SessionTitle: "Caching", Difficulty: quiz.DifficultyHard, QuestionCount: 3, CreatedAt: older.CreatedAt, AttemptCount: 2, LatestScore: &latest},
	}, got)
}

func TestQuizRepository_ListQuizzes_emptyIsAnEmptyList(t *testing.T) {
	got, err := newTestQuizRepository(t).ListQuizzes(context.Background())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestQuizRepository_DeleteQuiz_removesQuestionsAttemptsAndAnswers(t *testing.T) {
	// Given a quiz with an attempt, and another quiz that must survive
	db := newTestDB(t)
	repo := NewQuizRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-2", quizEpoch)))
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a1", "quiz-1", 3.3, quizEpoch)))
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a2", "quiz-2", 3.3, quizEpoch)))

	// When deleting the first
	require.NoError(t, repo.DeleteQuiz(ctx, "quiz-1"))

	// Then everything under it is gone and the other quiz is untouched
	for table, want := range map[string]int{"quizzes": 1, "quiz_questions": 3, "quiz_attempts": 1, "quiz_answers": 3} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&count))
		assert.Equalf(t, want, count, "table %s", table)
	}
}

func TestQuizRepository_DeleteQuiz_unknownIDIsNotFound(t *testing.T) {
	err := newTestQuizRepository(t).DeleteQuiz(context.Background(), "nope")

	assert.ErrorIs(t, err, quiz.ErrQuizNotFound)
}

func TestQuizRepository_CreateAttemptThenGetAttempt_roundTripsBlankAnswers(t *testing.T) {
	// Given a stored quiz and an attempt with a blank answer
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
	original := newTestQuizAttempt("a1", "quiz-1", 3.3, quizEpoch.Add(time.Hour))

	// When storing and reading it back
	require.NoError(t, repo.CreateAttempt(ctx, original))
	got, err := repo.GetAttempt(ctx, "a1")

	// Then every field survives, the answers in question order and the blank one as nil
	require.NoError(t, err)
	assert.Equal(t, original, got)
	assert.Nil(t, got.Answers[2].SelectedIndex)
}

func TestQuizRepository_CreateAttempt_isAllOrNothing(t *testing.T) {
	// Given an attempt that answers a question the quiz does not have
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
	attempt := newTestQuizAttempt("a1", "quiz-1", 3.3, quizEpoch)
	attempt.Answers[2].QuestionID = "ghost"

	// When storing it
	err := repo.CreateAttempt(ctx, attempt)

	// Then it fails and the attempt row is not left behind
	require.Error(t, err)
	_, err = repo.GetAttempt(ctx, "a1")
	assert.ErrorIs(t, err, quiz.ErrAttemptNotFound)
}

func TestQuizRepository_CreateAttempt_unknownQuizFails(t *testing.T) {
	err := newTestQuizRepository(t).CreateAttempt(context.Background(), newTestQuizAttempt("a1", "ghost", 1, quizEpoch))

	assert.Error(t, err)
}

func TestQuizRepository_GetAttempt_unknownIDIsNotFound(t *testing.T) {
	_, err := newTestQuizRepository(t).GetAttempt(context.Background(), "nope")

	assert.ErrorIs(t, err, quiz.ErrAttemptNotFound)
}

func TestQuizRepository_ListAttempts_newestFirstOfOneQuiz(t *testing.T) {
	// Given two attempts of one quiz and one of another
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-2", quizEpoch)))
	first := newTestQuizAttempt("a1", "quiz-1", 3.3, quizEpoch.Add(time.Hour))
	second := newTestQuizAttempt("a2", "quiz-1", 6.7, quizEpoch.Add(2*time.Hour))
	require.NoError(t, repo.CreateAttempt(ctx, first))
	require.NoError(t, repo.CreateAttempt(ctx, second))
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a3", "quiz-2", 1, quizEpoch)))

	// When listing the first quiz's attempts
	got, err := repo.ListAttempts(ctx, "quiz-1")

	// Then only its attempts come back, newest first, each with its answers
	require.NoError(t, err)
	assert.Equal(t, []quiz.Attempt{second, first}, got)
}

func TestQuizRepository_ListAttempts_noneIsAnEmptyList(t *testing.T) {
	got, err := newTestQuizRepository(t).ListAttempts(context.Background(), "quiz-1")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestQuizRepository_reads_failOnCorruptRows(t *testing.T) {
	ctx := context.Background()
	t.Run("options that are not a JSON list", func(t *testing.T) {
		db := newTestDB(t)
		repo := NewQuizRepository(db)
		require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
		_, err := db.Exec(`UPDATE quiz_questions SET options = 'not json' WHERE id = 'quiz-1-q1'`)
		require.NoError(t, err)

		_, err = repo.GetQuiz(ctx, "quiz-1")

		assert.Error(t, err)
	})
	t.Run("a quiz time that is not a time", func(t *testing.T) {
		db := newTestDB(t)
		repo := NewQuizRepository(db)
		require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
		_, err := db.Exec(`UPDATE quizzes SET created_at = 'yesterday'`)
		require.NoError(t, err)

		_, getErr := repo.GetQuiz(ctx, "quiz-1")
		_, listErr := repo.ListQuizzes(ctx)

		assert.Error(t, getErr)
		assert.Error(t, listErr)
	})
	t.Run("an attempt time that is not a time", func(t *testing.T) {
		db := newTestDB(t)
		repo := NewQuizRepository(db)
		require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-1", quizEpoch)))
		require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a1", "quiz-1", 3.3, quizEpoch)))
		_, err := db.Exec(`UPDATE quiz_attempts SET finished_at = 'yesterday'`)
		require.NoError(t, err)

		_, getErr := repo.GetAttempt(ctx, "a1")
		_, listErr := repo.ListAttempts(ctx, "quiz-1")

		assert.Error(t, getErr)
		assert.Error(t, listErr)
	})
}

func TestQuizRepository_CreateQuizThenGetQuiz_roundTripsTheMessageID(t *testing.T) {
	// Given a quiz an offer produced under a chat message
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	original := newTestQuiz("quiz-1", quizEpoch)
	original.MessageID = "message-1"

	// When storing it and reading it back
	require.NoError(t, repo.CreateQuiz(ctx, original))
	got, err := repo.GetQuiz(ctx, "quiz-1")

	// Then the message id is preserved, and a quiz from the menu has none
	require.NoError(t, err)
	assert.Equal(t, "message-1", got.MessageID)
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-2", quizEpoch)))
	menu, err := repo.GetQuiz(ctx, "quiz-2")
	require.NoError(t, err)
	assert.Empty(t, menu.MessageID)
}

func TestQuizRepository_FindByMessage_returnsTheQuizTheOfferProduced(t *testing.T) {
	// Given two offer quizzes and one from the menu, the first with an attempt
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	first, second := newTestQuiz("quiz-1", quizEpoch), newTestQuiz("quiz-2", quizEpoch.Add(time.Hour))
	first.MessageID, second.MessageID = "message-1", "message-2"
	require.NoError(t, repo.CreateQuiz(ctx, first))
	require.NoError(t, repo.CreateQuiz(ctx, second))
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-menu", quizEpoch)))
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a1", "quiz-1", 3.3, quizEpoch.Add(2*time.Hour))))

	// When finding by message
	got, err := repo.FindByMessage(ctx, "message-1")

	// Then it is that quiz's summary, with its attempt data
	require.NoError(t, err)
	latest := 3.3
	assert.Equal(t, quiz.Summary{
		ID: "quiz-1", SessionID: "session-gone", SessionTitle: "Caching", Difficulty: quiz.DifficultyHard,
		QuestionCount: 3, CreatedAt: first.CreatedAt, AttemptCount: 1, LatestScore: &latest, MessageID: "message-1",
	}, got)
}

func TestQuizRepository_FindByMessage_isNotFoundForAnUnknownOrEmptyMessage(t *testing.T) {
	// Given a quiz that came from the menu, with no message
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateQuiz(ctx, newTestQuiz("quiz-menu", quizEpoch)))

	// When finding by an unknown message and by the empty one
	_, unknown := repo.FindByMessage(ctx, "message-9")
	_, empty := repo.FindByMessage(ctx, "")

	// Then neither matches, so the empty id never finds a menu quiz
	assert.ErrorIs(t, unknown, quiz.ErrQuizNotFound)
	assert.ErrorIs(t, empty, quiz.ErrQuizNotFound)
}

func TestQuizRepository_ListBySession_returnsOnlyTheOfferQuizzesOfThatSessionOldestFirst(t *testing.T) {
	// Given offer quizzes in two sessions and a menu quiz in the first
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	older, newer, other, menu := newTestQuiz("quiz-old", quizEpoch), newTestQuiz("quiz-new", quizEpoch.Add(time.Hour)),
		newTestQuiz("quiz-other", quizEpoch), newTestQuiz("quiz-menu", quizEpoch)
	older.MessageID, newer.MessageID, other.MessageID = "message-1", "message-2", "message-3"
	other.SessionID = "session-2"
	for _, q := range []quiz.Quiz{newer, other, menu, older} {
		require.NoError(t, repo.CreateQuiz(ctx, q))
	}
	require.NoError(t, repo.CreateAttempt(ctx, newTestQuizAttempt("a1", "quiz-new", 8.0, quizEpoch.Add(2*time.Hour))))

	// When listing the first session's quizzes
	got, err := repo.ListBySession(ctx, "session-gone")

	// Then the two offer quizzes come back, the oldest first, with their attempt data
	require.NoError(t, err)
	latest := 8.0
	assert.Equal(t, []quiz.Summary{
		{ID: "quiz-old", SessionID: "session-gone", SessionTitle: "Caching", Difficulty: quiz.DifficultyHard, QuestionCount: 3, CreatedAt: older.CreatedAt, MessageID: "message-1"},
		{ID: "quiz-new", SessionID: "session-gone", SessionTitle: "Caching", Difficulty: quiz.DifficultyHard, QuestionCount: 3, CreatedAt: newer.CreatedAt, AttemptCount: 1, LatestScore: &latest, MessageID: "message-2"},
	}, got)
}

func TestQuizRepository_ListBySession_noneIsAnEmptyList(t *testing.T) {
	got, err := newTestQuizRepository(t).ListBySession(context.Background(), "session-1")

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestQuizRepository_ListQuizzes_includesTheMessageID(t *testing.T) {
	// Given an offer quiz
	repo := newTestQuizRepository(t)
	ctx := context.Background()
	offered := newTestQuiz("quiz-1", quizEpoch)
	offered.MessageID = "message-1"
	require.NoError(t, repo.CreateQuiz(ctx, offered))

	// When listing every quiz
	got, err := repo.ListQuizzes(ctx)

	// Then the summary carries the message id
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "message-1", got[0].MessageID)
}

func TestQuizRepository_FindByMessageAndListBySession_failOnACorruptTime(t *testing.T) {
	// Given an offer quiz with a corrupt creation time
	db := newTestDB(t)
	repo := NewQuizRepository(db)
	ctx := context.Background()
	offered := newTestQuiz("quiz-1", quizEpoch)
	offered.MessageID = "message-1"
	require.NoError(t, repo.CreateQuiz(ctx, offered))
	_, err := db.Exec(`UPDATE quizzes SET created_at = 'yesterday'`)
	require.NoError(t, err)

	// When reading it by message and by session
	_, findErr := repo.FindByMessage(ctx, "message-1")
	_, listErr := repo.ListBySession(ctx, "session-gone")

	// Then both fail
	assert.Error(t, findErr)
	assert.Error(t, listErr)
}
