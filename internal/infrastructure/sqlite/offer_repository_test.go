package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/exercise"
)

func newTestOfferRepository(t *testing.T) (*OfferRepository, *sql.DB) {
	t.Helper()
	_, db := newTestChallengeRepository(t)
	for _, id := range []string{"message-1", "message-2"} {
		_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, 'session-1', 'assistant', 'x', CURRENT_TIMESTAMP)`, id)
		require.NoError(t, err)
	}
	return NewOfferRepository(db), db
}

func newTestOffer(id, messageID string) exercise.Offer {
	return exercise.Offer{
		ID:        id,
		SessionID: "session-1",
		MessageID: messageID,
		CreatedAt: time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC),
	}
}

func TestOfferRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given an offer
	repo, _ := newTestOfferRepository(t)
	ctx := context.Background()
	offer := newTestOffer("offer-1", "message-1")

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

func TestOfferRepository_Create_refusesASecondOfferForTheSession(t *testing.T) {
	// Given a session with a pending offer
	repo, _ := newTestOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestOffer("offer-1", "message-1")))

	// When creating another offer for the same session
	err := repo.Create(ctx, newTestOffer("offer-2", "message-2"))

	// Then ErrOfferPending is returned
	assert.ErrorIs(t, err, exercise.ErrOfferPending)
}

func TestOfferRepository_Create_doesNotReportAnIDClashAsPending(t *testing.T) {
	// Given an offer in session-1 and a second session with a message
	repo, db := newTestOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestOffer("offer-1", "message-1")))
	insertTestSession(t, db, "session-2", "queues")
	_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-3', 'session-2', 'assistant', 'x', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	// When the other session's offer reuses the id
	other := exercise.Offer{ID: "offer-1", SessionID: "session-2", MessageID: "message-3", CreatedAt: time.Now().UTC()}
	err = repo.Create(ctx, other)

	// Then it fails, but it is not the "pending" case
	require.Error(t, err)
	assert.NotErrorIs(t, err, exercise.ErrOfferPending)
}

func TestOfferRepository_Get_returnsNotFound(t *testing.T) {
	// Given no offers
	repo, _ := newTestOfferRepository(t)

	// When reading by id and by session
	_, errID := repo.Get(context.Background(), "missing")
	_, errSession := repo.GetBySession(context.Background(), "session-1")

	// Then ErrOfferNotFound is returned for both
	assert.ErrorIs(t, errID, exercise.ErrOfferNotFound)
	assert.ErrorIs(t, errSession, exercise.ErrOfferNotFound)
}

func TestOfferRepository_Delete_removesTheOfferAndAllowsANewOne(t *testing.T) {
	// Given a pending offer
	repo, _ := newTestOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestOffer("offer-1", "message-1")))

	// When deleting it
	require.NoError(t, repo.Delete(ctx, "offer-1"))

	// Then it is gone and the session can get another one
	_, err := repo.Get(ctx, "offer-1")
	assert.ErrorIs(t, err, exercise.ErrOfferNotFound)
	assert.NoError(t, repo.Create(ctx, newTestOffer("offer-2", "message-2")))
}

func TestOfferRepository_Delete_ignoresAnUnknownOffer(t *testing.T) {
	// Given no offers
	repo, _ := newTestOfferRepository(t)

	// When deleting an unknown id
	// Then nothing fails, so resolving twice is harmless
	assert.NoError(t, repo.Delete(context.Background(), "missing"))
}

func TestOfferRepository_offerDisappearsWithItsMessage(t *testing.T) {
	// Given an offer under a message
	repo, db := newTestOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestOffer("offer-1", "message-1")))

	// When the message is deleted
	_, err := db.Exec(`DELETE FROM messages WHERE id = 'message-1'`)
	require.NoError(t, err)

	// Then the offer goes with it
	_, err = repo.Get(ctx, "offer-1")
	assert.ErrorIs(t, err, exercise.ErrOfferNotFound)
}

func TestChallengeRepository_FindLatestBySourceSession_returnsTheMostRecentOne(t *testing.T) {
	// Given two challenges from session-1, the second created later, and one from nowhere
	repo, db := newTestChallengeRepository(t)
	ctx := context.Background()
	older := newTestChallenge("challenge-old")
	newer := newTestChallenge("challenge-new")
	newer.CreatedAt = older.CreatedAt.Add(time.Hour)
	newer.UpdatedAt = newer.CreatedAt
	manual := newTestChallenge("challenge-manual")
	manual.SourceSessionID = ""
	manual.CreatedAt = newer.CreatedAt.Add(time.Hour)
	for _, c := range []exercise.Challenge{older, newer, manual} {
		require.NoError(t, repo.Create(ctx, c))
	}
	insertTestSession(t, db, "session-2", "queues")

	// When finding the latest for session-1 and for a session with none
	got, err := repo.FindLatestBySourceSession(ctx, "session-1")
	_, errNone := repo.FindLatestBySourceSession(ctx, "session-2")

	// Then the newest of its own comes back, and none is not found
	require.NoError(t, err)
	assert.Equal(t, "challenge-new", got.ID)
	assert.ErrorIs(t, errNone, exercise.ErrChallengeNotFound)
}
