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

var mindMapEpoch = time.Date(2026, 11, 17, 9, 0, 0, 123456789, time.UTC)

func newTestMindMapRepository(t *testing.T) (*MindMapRepository, *sql.DB) {
	t.Helper()
	_, db := newTestChallengeRepository(t) // seeds folder-a and session-1
	insertTestSession(t, db, "session-2", "queues")
	return NewMindMapRepository(db), db
}

func newTestMindMap(id, sessionID string, at time.Time) mindmap.MindMap {
	return mindmap.MindMap{
		ID: id, SessionID: sessionID, SessionTitle: "Caching", Title: "Caching " + id,
		Tree: `{"label":"Caching","children":[{"label":"LRU"}]}`, DiagramXML: "<mxGraphModel/>",
		CreatedAt: at, UpdatedAt: at.Add(time.Minute),
	}
}

func TestMindMapRepository_CreateThenGet_roundTripsEveryField(t *testing.T) {
	// Given a mind map born from a chat offer
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	original := newTestMindMap("map-1", "session-1", mindMapEpoch)
	original.MessageID = "message-1"

	// When storing it and reading it back
	require.NoError(t, repo.Create(ctx, original))
	got, err := repo.Get(ctx, "map-1")

	// Then every field is preserved
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestMindMapRepository_CreateThenGet_keepsAMenuMapWithoutAMessage(t *testing.T) {
	// Given a mind map generated from the menu
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	original := newTestMindMap("map-1", "session-1", mindMapEpoch)

	// When storing it and reading it back
	require.NoError(t, repo.Create(ctx, original))
	got, err := repo.Get(ctx, "map-1")

	// Then it has no message id
	require.NoError(t, err)
	assert.Empty(t, got.MessageID)
}

func TestMindMapRepository_Create_refusesADuplicateID(t *testing.T) {
	// Given a stored map
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch)))

	// When storing another with the same id
	err := repo.Create(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch))

	// Then it fails
	assert.Error(t, err)
}

func TestMindMapRepository_Get_returnsNotFound(t *testing.T) {
	// Given no maps
	repo, _ := newTestMindMapRepository(t)

	// When reading an unknown id
	_, err := repo.Get(context.Background(), "missing")

	// Then ErrMindMapNotFound is returned
	assert.ErrorIs(t, err, mindmap.ErrMindMapNotFound)
}

func TestMindMapRepository_Get_failsOnACorruptTime(t *testing.T) {
	corruptions := map[string]string{
		"created_at": `UPDATE mind_maps SET created_at = 'yesterday'`,
		"updated_at": `UPDATE mind_maps SET updated_at = 'yesterday'`,
	}
	for column, corrupt := range corruptions {
		t.Run(column, func(t *testing.T) {
			// Given a map with a corrupt time
			repo, db := newTestMindMapRepository(t)
			ctx := context.Background()
			require.NoError(t, repo.Create(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch)))
			_, err := db.Exec(corrupt)
			require.NoError(t, err)

			// When reading it, and listing it
			_, errGet := repo.Get(ctx, "map-1")
			_, errList := repo.List(ctx)

			// Then both fail, and not as "not found"
			require.Error(t, errGet)
			assert.NotErrorIs(t, errGet, mindmap.ErrMindMapNotFound)
			assert.ErrorContains(t, errGet, `reading mind map "map-1"`)
			assert.ErrorContains(t, errList, `reading mind map "map-1"`)
		})
	}
}

func TestMindMapRepository_List_returnsNewestFirstWithoutTheDiagram(t *testing.T) {
	// Given three maps created at different times
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	for _, m := range []mindmap.MindMap{
		newTestMindMap("map-old", "session-1", mindMapEpoch),
		newTestMindMap("map-new", "session-2", mindMapEpoch.Add(2*time.Hour)),
		newTestMindMap("map-mid", "session-1", mindMapEpoch.Add(time.Hour)),
	} {
		require.NoError(t, repo.Create(ctx, m))
	}

	// When listing
	got, err := repo.List(ctx)

	// Then they come newest first, as summaries
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"map-new", "map-mid", "map-old"}, []string{got[0].ID, got[1].ID, got[2].ID})
	assert.Equal(t, mindmap.Summary{
		ID: "map-new", SessionID: "session-2", SessionTitle: "Caching", Title: "Caching map-new",
		CreatedAt: mindMapEpoch.Add(2 * time.Hour), UpdatedAt: mindMapEpoch.Add(2*time.Hour + time.Minute),
	}, got[0])
}

func TestMindMapRepository_List_isEmptyWithoutMaps(t *testing.T) {
	// Given no maps
	repo, _ := newTestMindMapRepository(t)

	// When listing
	got, err := repo.List(context.Background())

	// Then the list is empty and not nil, so it serializes as [] and not null
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestMindMapRepository_FindByMessage(t *testing.T) {
	// Given a map from an offer and a map from the menu
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	fromOffer := newTestMindMap("map-1", "session-1", mindMapEpoch)
	fromOffer.MessageID = "message-1"
	require.NoError(t, repo.Create(ctx, fromOffer))
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-2", "session-1", mindMapEpoch.Add(time.Hour))))

	// When finding by the message, by an unknown message and by an empty one
	found, errFound := repo.FindByMessage(ctx, "message-1")
	_, errUnknown := repo.FindByMessage(ctx, "message-9")
	_, errEmpty := repo.FindByMessage(ctx, "")

	// Then the offer's map is found and nothing else matches, the menu map included
	require.NoError(t, errFound)
	assert.Equal(t, "map-1", found.ID)
	assert.Equal(t, "message-1", found.MessageID)
	assert.ErrorIs(t, errUnknown, mindmap.ErrMindMapNotFound)
	assert.ErrorIs(t, errEmpty, mindmap.ErrMindMapNotFound)
}

func TestMindMapRepository_FindByMessage_returnsTheOldestWhenThereAreTwo(t *testing.T) {
	// Given two maps linked to the same message
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	for id, at := range map[string]time.Time{"map-late": mindMapEpoch.Add(time.Hour), "map-early": mindMapEpoch} {
		m := newTestMindMap(id, "session-1", at)
		m.MessageID = "message-1"
		require.NoError(t, repo.Create(ctx, m))
	}

	// When finding by the message
	found, err := repo.FindByMessage(ctx, "message-1")

	// Then the oldest is returned
	require.NoError(t, err)
	assert.Equal(t, "map-early", found.ID)
}

func TestMindMapRepository_ListBySession_returnsOfferMapsOldestFirst(t *testing.T) {
	// Given offer maps in two sessions, in the wrong order, and a menu map
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	add := func(id, session, message string, at time.Time) {
		m := newTestMindMap(id, session, at)
		m.MessageID = message
		require.NoError(t, repo.Create(ctx, m))
	}
	add("map-b", "session-1", "message-2", mindMapEpoch.Add(time.Hour))
	add("map-a", "session-1", "message-1", mindMapEpoch)
	add("map-other", "session-2", "message-3", mindMapEpoch)
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-menu", "session-1", mindMapEpoch)))

	// When listing the first session's maps
	got, err := repo.ListBySession(ctx, "session-1")

	// Then only its offer maps come, oldest first
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "map-a", got[0].ID)
	assert.Equal(t, "map-b", got[1].ID)
}

func TestMindMapRepository_Update_replacesTheContentAndKeepsTheRest(t *testing.T) {
	// Given a stored offer map
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	original := newTestMindMap("map-1", "session-1", mindMapEpoch)
	original.MessageID = "message-1"
	require.NoError(t, repo.Create(ctx, original))
	changed := original
	changed.SessionTitle = "New title"
	changed.Title = "New"
	changed.Tree = `{"label":"New","children":[{"label":"x"}]}`
	changed.DiagramXML = "<mxGraphModel>new</mxGraphModel>"
	changed.UpdatedAt = mindMapEpoch.Add(24 * time.Hour)
	changed.SessionID = "session-2"         // not updatable
	changed.MessageID = "message-other"     // not updatable
	changed.CreatedAt = mindMapEpoch.Add(1) // not updatable

	// When updating it
	require.NoError(t, repo.Update(ctx, changed))
	got, err := repo.Get(ctx, "map-1")

	// Then the content and the update time change; session, message and creation time stay
	require.NoError(t, err)
	want := changed
	want.SessionID = original.SessionID
	want.MessageID = original.MessageID
	want.CreatedAt = original.CreatedAt
	assert.Equal(t, want, got)
}

func TestMindMapRepository_Update_returnsNotFound(t *testing.T) {
	// Given no maps
	repo, _ := newTestMindMapRepository(t)

	// When updating an unknown map
	err := repo.Update(context.Background(), newTestMindMap("missing", "session-1", mindMapEpoch))

	// Then ErrMindMapNotFound is returned
	assert.ErrorIs(t, err, mindmap.ErrMindMapNotFound)
}

func TestMindMapRepository_Delete_removesTheMap(t *testing.T) {
	// Given two stored maps
	repo, _ := newTestMindMapRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch)))
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-2", "session-1", mindMapEpoch)))

	// When deleting one, and then deleting it again
	require.NoError(t, repo.Delete(ctx, "map-1"))
	err := repo.Delete(ctx, "map-1")

	// Then only that map is gone and a second delete is not found
	_, errGone := repo.Get(ctx, "map-1")
	_, errKept := repo.Get(ctx, "map-2")
	assert.ErrorIs(t, errGone, mindmap.ErrMindMapNotFound)
	assert.NoError(t, errKept)
	assert.ErrorIs(t, err, mindmap.ErrMindMapNotFound)
}

func TestMindMapRepository_outlivesItsSession(t *testing.T) {
	// Given a map in each of two sessions
	repo, db := newTestMindMapRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch)))
	require.NoError(t, repo.Create(ctx, newTestMindMap("map-2", "session-2", mindMapEpoch)))

	// When the first session is deleted
	_, err := db.Exec(`DELETE FROM sessions WHERE id = 'session-1'`)
	require.NoError(t, err)

	// Then both maps stay, like a quiz: only an explicit delete removes one
	got, errFirst := repo.Get(ctx, "map-1")
	_, errSecond := repo.Get(ctx, "map-2")
	require.NoError(t, errFirst)
	assert.NoError(t, errSecond)
	assert.Equal(t, "session-1", got.SessionID)
}

func TestMindMapRepository_failsOnAClosedDatabase(t *testing.T) {
	// Given a repository whose database is closed
	repo, db := newTestMindMapRepository(t)
	require.NoError(t, db.Close())
	ctx := context.Background()

	// When using every method
	_, errGet := repo.Get(ctx, "map-1")
	_, errList := repo.List(ctx)
	_, errFind := repo.FindByMessage(ctx, "message-1")
	_, errSession := repo.ListBySession(ctx, "session-1")

	// Then each fails with a wrapped error, never as "not found"
	for _, err := range []error{errGet, errList, errFind, errSession,
		repo.Create(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch)),
		repo.Update(ctx, newTestMindMap("map-1", "session-1", mindMapEpoch)),
		repo.Delete(ctx, "map-1")} {
		require.Error(t, err)
		assert.NotErrorIs(t, err, mindmap.ErrMindMapNotFound)
	}
}
