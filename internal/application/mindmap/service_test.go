package mindmap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
	domainmocks "github.com/santaniello/athena/internal/domain/mindmap/mocks"
)

func newService(t *testing.T) (*Service, *domainmocks.MockRepository) {
	t.Helper()
	repo := domainmocks.NewMockRepository(t)
	service := NewService(repo)
	service.now = func() time.Time { return fixedNow }
	return service, repo
}

func TestService_ListMindMaps(t *testing.T) {
	// Given a repository with two maps
	service, repo := newService(t)
	ctx := context.Background()
	want := []domainmindmap.Summary{{ID: "a"}, {ID: "b"}}
	repo.EXPECT().List(ctx).Return(want, nil).Once()

	// When the catalog is listed
	got, err := service.ListMindMaps(ctx)

	// Then the summaries come back as they are
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestService_ListMindMaps_wrapsAFailure(t *testing.T) {
	// Given a repository that fails
	service, repo := newService(t)
	ctx := context.Background()
	boom := errors.New("boom")
	repo.EXPECT().List(ctx).Return(nil, boom).Once()

	// When the catalog is listed
	_, err := service.ListMindMaps(ctx)

	// Then the failure is wrapped
	assert.ErrorIs(t, err, boom)
}

func TestService_GetMindMap(t *testing.T) {
	// Given a stored map
	service, repo := newService(t)
	ctx := context.Background()
	want := expectedMindMap(t)
	repo.EXPECT().Get(ctx, "map-1").Return(want, nil).Once()

	// When it is read
	got, err := service.GetMindMap(ctx, "map-1")

	// Then it comes back with its tree and diagram
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestService_GetMindMap_reportsNotFound(t *testing.T) {
	// Given no such map
	service, repo := newService(t)
	ctx := context.Background()
	repo.EXPECT().Get(ctx, "nope").Return(domainmindmap.MindMap{}, domainmindmap.ErrMindMapNotFound).Once()

	// When it is read
	_, err := service.GetMindMap(ctx, "nope")

	// Then the not-found error is kept
	assert.ErrorIs(t, err, domainmindmap.ErrMindMapNotFound)
}

func TestService_SaveDiagram_replacesTheDiagramAndStampsTheTime(t *testing.T) {
	// Given a stored map and a hand-edited draw.io model
	service, repo := newService(t)
	ctx := context.Background()
	stored := storedMindMap(t)
	edited := `<mxGraphModel><root><mxCell id="0"/></root></mxGraphModel>`
	want := stored
	want.DiagramXML = edited
	want.UpdatedAt = fixedNow
	repo.EXPECT().Get(ctx, "map-1").Return(stored, nil).Once()
	repo.EXPECT().Update(ctx, want).Return(nil).Once()

	// When the diagram is saved
	got, err := service.SaveDiagram(ctx, "map-1", edited)

	// Then only the diagram and the update time change; the tree stays
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, stored.Tree, got.Tree)
}

func TestService_SaveDiagram_acceptsADrawioFile(t *testing.T) {
	// Given a stored map and an mxfile document
	service, repo := newService(t)
	ctx := context.Background()
	stored := storedMindMap(t)
	repo.EXPECT().Get(ctx, "map-1").Return(stored, nil).Once()
	repo.EXPECT().Update(ctx, withDiagram(stored, `<mxfile><diagram/></mxfile>`)).Return(nil).Once()

	// When the diagram is saved
	_, err := service.SaveDiagram(ctx, "map-1", `<mxfile><diagram/></mxfile>`)

	// Then it is accepted
	assert.NoError(t, err)
}

func TestService_SaveDiagram_rejectsWhatIsNotDrawio(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"not XML", "hello"},
		{"another root element", "<html><body/></html>"},
		{"broken XML", "<mxGraphModel>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a stored map and a document that is not draw.io
			service, repo := newService(t)
			ctx := context.Background()
			repo.EXPECT().Get(ctx, "map-1").Return(storedMindMap(t), nil).Once()

			// When the diagram is saved
			_, err := service.SaveDiagram(ctx, "map-1", tt.text)

			// Then it is refused and nothing is updated (the mock asserts it)
			assert.ErrorIs(t, err, ErrInvalidDiagram)
		})
	}
}

func TestService_SaveDiagram_rejectsAnEmptyOrOversizedDiagram(t *testing.T) {
	tests := []struct {
		name string
		text string
		want error
	}{
		{"empty", "", domainmindmap.ErrDiagramRequired},
		{"over the cap", "<mxGraphModel>" + strings.Repeat("x", domainmindmap.MaxDiagramBytes) + "</mxGraphModel>", domainmindmap.ErrDiagramTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a stored map
			service, repo := newService(t)
			ctx := context.Background()
			repo.EXPECT().Get(ctx, "map-1").Return(storedMindMap(t), nil).Once()

			// When the diagram is saved
			_, err := service.SaveDiagram(ctx, "map-1", tt.text)

			// Then the domain rule refuses it
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestService_SaveDiagram_reportsNotFound(t *testing.T) {
	// Given no such map
	service, repo := newService(t)
	ctx := context.Background()
	repo.EXPECT().Get(ctx, "nope").Return(domainmindmap.MindMap{}, domainmindmap.ErrMindMapNotFound).Once()

	// When the diagram is saved
	_, err := service.SaveDiagram(ctx, "nope", "<mxGraphModel/>")

	// Then it is not found
	assert.ErrorIs(t, err, domainmindmap.ErrMindMapNotFound)
}

func TestService_SaveDiagram_wrapsAnUpdateFailure(t *testing.T) {
	// Given a repository whose update fails
	service, repo := newService(t)
	ctx := context.Background()
	boom := errors.New("boom")
	stored := storedMindMap(t)
	repo.EXPECT().Get(ctx, "map-1").Return(stored, nil).Once()
	repo.EXPECT().Update(ctx, withDiagram(stored, "<mxGraphModel/>")).Return(boom).Once()

	// When the diagram is saved
	_, err := service.SaveDiagram(ctx, "map-1", "<mxGraphModel/>")

	// Then the failure is returned
	assert.ErrorIs(t, err, boom)
}

func TestService_DeleteMindMap(t *testing.T) {
	// Given a stored map
	service, repo := newService(t)
	ctx := context.Background()
	repo.EXPECT().Delete(ctx, "map-1").Return(nil).Once()

	// When it is deleted
	err := service.DeleteMindMap(ctx, "map-1")

	// Then it succeeds
	assert.NoError(t, err)
}

func TestService_DeleteMindMap_reportsNotFound(t *testing.T) {
	// Given no such map
	service, repo := newService(t)
	ctx := context.Background()
	repo.EXPECT().Delete(ctx, "nope").Return(domainmindmap.ErrMindMapNotFound).Once()

	// When it is deleted
	err := service.DeleteMindMap(ctx, "nope")

	// Then the not-found error is kept
	assert.ErrorIs(t, err, domainmindmap.ErrMindMapNotFound)
}

func TestService_ListSessionMindMaps(t *testing.T) {
	// Given a repository with the session's maps
	service, repo := newService(t)
	ctx := context.Background()
	want := []domainmindmap.Summary{{ID: "a", MessageID: "m1"}}
	repo.EXPECT().ListBySession(ctx, "session-1").Return(want, nil).Once()

	// When the session's maps are listed
	got, err := service.ListSessionMindMaps(ctx, "session-1")

	// Then they come back
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestService_ListSessionMindMaps_wrapsAFailure(t *testing.T) {
	// Given a repository that fails
	service, repo := newService(t)
	ctx := context.Background()
	boom := errors.New("boom")
	repo.EXPECT().ListBySession(ctx, "session-1").Return(nil, boom).Once()

	// When the session's maps are listed
	_, err := service.ListSessionMindMaps(ctx, "session-1")

	// Then the failure is wrapped
	assert.ErrorIs(t, err, boom)
}

func TestNewService_usesTheRealClock(t *testing.T) {
	// Given a service built with the public constructor
	service := NewService(domainmocks.NewMockRepository(t))

	// When its clock is read
	now := service.now()

	// Then it is UTC and recent
	assert.WithinDuration(t, time.Now(), now, time.Minute)
}

// withDiagram is the stored map as SaveDiagram updates it: the new diagram and the update time.
func withDiagram(stored domainmindmap.MindMap, diagram string) domainmindmap.MindMap {
	stored.DiagramXML = diagram
	stored.UpdatedAt = fixedNow
	return stored
}
