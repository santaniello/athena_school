package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/quiz"
)

func newTestQuizOfferRepository(t *testing.T) (*QuizOfferRepository, *sql.DB) {
	t.Helper()
	_, db := newTestChallengeRepository(t)
	for _, id := range []string{"message-1", "message-2"} {
		_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, 'session-1', 'assistant', 'x', CURRENT_TIMESTAMP)`, id)
		require.NoError(t, err)
	}
	return NewQuizOfferRepository(db), db
}

func newTestQuizOffer(id, messageID string) quiz.Offer {
	return quiz.Offer{
		ID:        id,
		SessionID: "session-1",
		MessageID: messageID,
		CreatedAt: time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC),
	}
}

func TestQuizOfferRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given an offer
	repo, _ := newTestQuizOfferRepository(t)
	ctx := context.Background()
	offer := newTestQuizOffer("offer-1", "message-1")

	// When creating it and reading it by id and by session
	require.NoError(t, repo.Create(ctx, offer))
	byID, errID := repo.Get(ctx, "offer-1")
	bySession, errSession := repo.GetBySession(ctx, "session-1")

	// Then every field is preserved
	require.NoError(t, errID)
	require.NoError(t, errSession)
	assert.Equal(t, offer, byID)
	assert.Equal(t, offer, bySession)
}

func TestQuizOfferRepository_Create_refusesASecondOfferForTheSession(t *testing.T) {
	// Given a session with a pending offer
	repo, _ := newTestQuizOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-1", "message-1")))

	// When creating another offer for the same session
	err := repo.Create(ctx, newTestQuizOffer("offer-2", "message-2"))

	// Then ErrOfferPending is returned
	assert.ErrorIs(t, err, quiz.ErrOfferPending)
}

func TestQuizOfferRepository_Create_doesNotReportAnIDClashAsPending(t *testing.T) {
	// Given an offer in session-1 and a second session with a message
	repo, db := newTestQuizOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-1", "message-1")))
	insertTestSession(t, db, "session-2", "queues")
	_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-3', 'session-2', 'assistant', 'x', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	// When the other session's offer reuses the id
	err = repo.Create(ctx, quiz.Offer{ID: "offer-1", SessionID: "session-2", MessageID: "message-3", CreatedAt: time.Now().UTC()})

	// Then it fails, but it is not the "pending" case
	require.Error(t, err)
	assert.NotErrorIs(t, err, quiz.ErrOfferPending)
}

func TestQuizOfferRepository_Get_returnsNotFound(t *testing.T) {
	// Given no offers
	repo, _ := newTestQuizOfferRepository(t)

	// When reading by id and by session
	_, errID := repo.Get(context.Background(), "missing")
	_, errSession := repo.GetBySession(context.Background(), "session-1")

	// Then ErrOfferNotFound is returned for both
	assert.ErrorIs(t, errID, quiz.ErrOfferNotFound)
	assert.ErrorIs(t, errSession, quiz.ErrOfferNotFound)
}

func TestQuizOfferRepository_Get_failsOnACorruptTime(t *testing.T) {
	// Given an offer with a corrupt creation time
	repo, db := newTestQuizOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-1", "message-1")))
	_, err := db.Exec(`UPDATE quiz_offers SET created_at = 'yesterday'`)
	require.NoError(t, err)

	// When reading it
	_, err = repo.Get(ctx, "offer-1")

	// Then it fails, and not as "not found"
	require.Error(t, err)
	assert.NotErrorIs(t, err, quiz.ErrOfferNotFound)
}

func TestQuizOfferRepository_Delete_removesTheOfferAndAllowsANewOne(t *testing.T) {
	// Given a pending offer
	repo, _ := newTestQuizOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-1", "message-1")))

	// When deleting it, and deleting an unknown id
	require.NoError(t, repo.Delete(ctx, "offer-1"))
	require.NoError(t, repo.Delete(ctx, "missing"))

	// Then it is gone and the session can get another one
	_, err := repo.Get(ctx, "offer-1")
	assert.ErrorIs(t, err, quiz.ErrOfferNotFound)
	assert.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-2", "message-2")))
}

func TestQuizOfferRepository_goesWithItsSessionAndItsMessage(t *testing.T) {
	t.Run("deleting the message", func(t *testing.T) {
		// Given a pending offer
		repo, db := newTestQuizOfferRepository(t)
		ctx := context.Background()
		require.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-1", "message-1")))

		// When its message is deleted
		_, err := db.Exec(`DELETE FROM messages WHERE id = 'message-1'`)
		require.NoError(t, err)

		// Then the offer is gone
		_, err = repo.Get(ctx, "offer-1")
		assert.ErrorIs(t, err, quiz.ErrOfferNotFound)
	})
	t.Run("deleting the session", func(t *testing.T) {
		// Given a pending offer
		repo, db := newTestQuizOfferRepository(t)
		ctx := context.Background()
		require.NoError(t, repo.Create(ctx, newTestQuizOffer("offer-1", "message-1")))

		// When its session is deleted
		_, err := db.Exec(`DELETE FROM sessions WHERE id = 'session-1'`)
		require.NoError(t, err)

		// Then the offer is gone
		_, err = repo.Get(ctx, "offer-1")
		assert.ErrorIs(t, err, quiz.ErrOfferNotFound)
	})
}

func TestOpen_addsTheMessageIDColumnToAQuizzesTableFromBeforeIt(t *testing.T) {
	// Given a database whose quizzes table predates the message_id column
	path := filepath.Join(t.TempDir(), "athena.db")
	legacy, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`CREATE TABLE quizzes (
		id TEXT PRIMARY KEY, session_id TEXT NOT NULL, session_title TEXT NOT NULL,
		difficulty TEXT NOT NULL, created_at TEXT NOT NULL
	);
	INSERT INTO quizzes (id, session_id, session_title, difficulty, created_at) VALUES ('quiz-1', 's', 'Go', 'EASY', 'now')`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	// When opening it twice (the migration is idempotent)
	db, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	db, err = Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Then the column exists and the old quiz has no message
	has, err := hasColumn(db, "quizzes", "message_id")
	require.NoError(t, err)
	assert.True(t, has)
	var messageID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT message_id FROM quizzes WHERE id = 'quiz-1'`).Scan(&messageID))
	assert.False(t, messageID.Valid)
}
