package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

func newTestDocumentRepository(t *testing.T) *DocumentRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, testSessionID)
	return NewDocumentRepository(db)
}

func TestDocumentRepository_Get_returnsErrDocumentNotFound_whenNoneIsStored(t *testing.T) {
	// Given a repository with no stored document
	repo := newTestDocumentRepository(t)

	// When getting a document that was never saved
	_, err := repo.Get(context.Background(), testSessionID, "item-never-imported")

	// Then it reports ErrDocumentNotFound
	assert.True(t, errors.Is(err, knowledge.ErrDocumentNotFound))
}

func TestDocumentRepository_Save_thenGet_roundTripsTheContent(t *testing.T) {
	// Given a repository and a document's full text
	repo := newTestDocumentRepository(t)
	ctx := context.Background()
	content := "# Heading\n\nBody text of the imported document."

	// When saving then getting it back
	require.NoError(t, repo.Save(ctx, "item-1", testSessionID, content))
	got, err := repo.Get(ctx, testSessionID, "item-1")

	// Then the exact text round-trips
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestDocumentRepository_Save_replacesThePreviousText_onASecondCallForTheSameItem(t *testing.T) {
	// Given a document already saved for an item
	repo := newTestDocumentRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "item-1", testSessionID, "old content"))

	// When saving new content for the same item (a re-import)
	require.NoError(t, repo.Save(ctx, "item-1", testSessionID, "new content"))
	got, err := repo.Get(ctx, testSessionID, "item-1")

	// Then the stored text is replaced, not duplicated
	require.NoError(t, err)
	assert.Equal(t, "new content", got)
}

func TestDocumentRepository_Get_returnsErrDocumentNotFound_whenTheItemBelongsToAnotherSession(t *testing.T) {
	// Given a document saved under one session
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewDocumentRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "item-1", "session-a", "content"))

	// When getting it by another session's ID
	_, getErr := repo.Get(ctx, "session-b", "item-1")

	// Then it is treated the same as not existing at all
	assert.True(t, errors.Is(getErr, knowledge.ErrDocumentNotFound))
}

func TestDocumentRepository_DeleteByItemID_removesTheStoredText(t *testing.T) {
	// Given a document saved for an item
	repo := newTestDocumentRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "item-1", testSessionID, "content"))

	// When deleting it
	require.NoError(t, repo.DeleteByItemID(ctx, testSessionID, "item-1"))

	// Then it is gone
	_, err := repo.Get(ctx, testSessionID, "item-1")
	assert.True(t, errors.Is(err, knowledge.ErrDocumentNotFound))
}

func TestDocumentRepository_DeleteByItemID_isNoOp_whenNothingIsStored(t *testing.T) {
	// Given a repository with no stored document
	repo := newTestDocumentRepository(t)

	// When deleting a document that was never saved
	err := repo.DeleteByItemID(context.Background(), testSessionID, "item-never-imported")

	// Then it succeeds without error
	assert.NoError(t, err)
}

func TestDocumentRepository_DeleteByItemID_leavesOtherItemsAndSessionsUntouched(t *testing.T) {
	// Given documents saved for two items, one in another session
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	seedSession(t, db, "session-a")
	seedSession(t, db, "session-b")
	repo := NewDocumentRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, "item-a", "session-a", "content a"))
	require.NoError(t, repo.Save(ctx, "item-b", "session-b", "content b"))

	// When deleting only item-a's document
	require.NoError(t, repo.DeleteByItemID(ctx, "session-a", "item-a"))

	// Then item-b's document survives untouched
	got, getErr := repo.Get(ctx, "session-b", "item-b")
	require.NoError(t, getErr)
	assert.Equal(t, "content b", got)
}
