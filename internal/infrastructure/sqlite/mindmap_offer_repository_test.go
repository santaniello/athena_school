package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/mindmap"
)

func newTestMindMapOfferRepository(t *testing.T) (*MindMapOfferRepository, *sql.DB) {
	t.Helper()
	_, db := newTestChallengeRepository(t)
	for _, id := range []string{"message-1", "message-2"} {
		_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, 'session-1', 'assistant', 'x', CURRENT_TIMESTAMP)`, id)
		require.NoError(t, err)
	}
	return NewMindMapOfferRepository(db), db
}

func newTestMindMapOffer(id, messageID string) mindmap.Offer {
	return mindmap.Offer{
		ID:        id,
		SessionID: "session-1",
		MessageID: messageID,
		CreatedAt: time.Date(2026, 11, 1, 10, 0, 0, 123456789, time.UTC),
	}
}

func TestMindMapOfferRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given an offer
	repo, _ := newTestMindMapOfferRepository(t)
	ctx := context.Background()
	offer := newTestMindMapOffer("offer-1", "message-1")

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

func TestMindMapOfferRepository_Create_refusesASecondOfferForTheSession(t *testing.T) {
	// Given a session with a pending offer
	repo, _ := newTestMindMapOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-1", "message-1")))

	// When creating another offer for the same session
	err := repo.Create(ctx, newTestMindMapOffer("offer-2", "message-2"))

	// Then ErrOfferPending is returned
	assert.ErrorIs(t, err, mindmap.ErrOfferPending)
}

func TestMindMapOfferRepository_Create_doesNotReportAnIDClashAsPending(t *testing.T) {
	// Given an offer in session-1 and a second session with a message
	repo, db := newTestMindMapOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-1", "message-1")))
	insertTestSession(t, db, "session-2", "queues")
	_, err := db.Exec(`INSERT INTO messages (id, session_id, role, content, created_at) VALUES ('message-3', 'session-2', 'assistant', 'x', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	// When the other session's offer reuses the id
	err = repo.Create(ctx, mindmap.Offer{ID: "offer-1", SessionID: "session-2", MessageID: "message-3", CreatedAt: time.Now().UTC()})

	// Then it fails, but it is not the "pending" case
	require.Error(t, err)
	assert.NotErrorIs(t, err, mindmap.ErrOfferPending)
}

func TestMindMapOfferRepository_Get_returnsNotFound(t *testing.T) {
	// Given no offers
	repo, _ := newTestMindMapOfferRepository(t)

	// When reading by id and by session
	_, errID := repo.Get(context.Background(), "missing")
	_, errSession := repo.GetBySession(context.Background(), "session-1")

	// Then ErrOfferNotFound is returned for both
	assert.ErrorIs(t, errID, mindmap.ErrOfferNotFound)
	assert.ErrorIs(t, errSession, mindmap.ErrOfferNotFound)
}

func TestMindMapOfferRepository_Get_failsOnACorruptTime(t *testing.T) {
	// Given an offer with a corrupt creation time
	repo, db := newTestMindMapOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-1", "message-1")))
	_, err := db.Exec(`UPDATE mind_map_offers SET created_at = 'yesterday'`)
	require.NoError(t, err)

	// When reading it
	_, err = repo.Get(ctx, "offer-1")

	// Then it fails, and not as "not found"
	require.Error(t, err)
	assert.NotErrorIs(t, err, mindmap.ErrOfferNotFound)
}

func TestMindMapOfferRepository_Delete_removesTheOfferAndAllowsANewOne(t *testing.T) {
	// Given a pending offer
	repo, _ := newTestMindMapOfferRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-1", "message-1")))

	// When deleting it, and deleting an unknown id
	require.NoError(t, repo.Delete(ctx, "offer-1"))
	require.NoError(t, repo.Delete(ctx, "missing"))

	// Then it is gone and the session can get another one
	_, err := repo.Get(ctx, "offer-1")
	assert.ErrorIs(t, err, mindmap.ErrOfferNotFound)
	assert.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-2", "message-2")))
}

func TestMindMapOfferRepository_goesWithItsSessionAndItsMessage(t *testing.T) {
	t.Run("deleting the message", func(t *testing.T) {
		// Given a pending offer
		repo, db := newTestMindMapOfferRepository(t)
		ctx := context.Background()
		require.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-1", "message-1")))

		// When its message is deleted
		_, err := db.Exec(`DELETE FROM messages WHERE id = 'message-1'`)
		require.NoError(t, err)

		// Then the offer is gone
		_, err = repo.Get(ctx, "offer-1")
		assert.ErrorIs(t, err, mindmap.ErrOfferNotFound)
	})
	t.Run("deleting the session", func(t *testing.T) {
		// Given a pending offer
		repo, db := newTestMindMapOfferRepository(t)
		ctx := context.Background()
		require.NoError(t, repo.Create(ctx, newTestMindMapOffer("offer-1", "message-1")))

		// When its session is deleted
		_, err := db.Exec(`DELETE FROM sessions WHERE id = 'session-1'`)
		require.NoError(t, err)

		// Then the offer is gone
		_, err = repo.Get(ctx, "offer-1")
		assert.ErrorIs(t, err, mindmap.ErrOfferNotFound)
	})
}
