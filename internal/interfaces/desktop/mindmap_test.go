package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationmindmap "github.com/santaniello/athena/internal/application/mindmap"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

var mindMapTime = time.Date(2026, 11, 17, 9, 30, 0, 0, time.UTC)

func mindMapCodeOf(t *testing.T, err error) MindMapError {
	t.Helper()
	var coded *MindMapError
	require.ErrorAs(t, err, &coded)
	var decoded MindMapError
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &decoded))
	assert.Equal(t, *coded, decoded)
	return decoded
}

func boundMindMap() domainmindmap.MindMap {
	return domainmindmap.MindMap{
		ID: "map-1", SessionID: "session-1", SessionTitle: "Caching", Title: "Caching basics",
		Tree: `{"label":"x","children":[{"label":"y"}]}`, DiagramXML: "<mxGraphModel/>",
		CreatedAt: mindMapTime, UpdatedAt: mindMapTime.Add(time.Hour), MessageID: "message-1",
	}
}

type fakeMindMapGenerator struct {
	mindMap     domainmindmap.MindMap
	err         error
	sessionID   string
	regenerated string
}

func (g *fakeMindMapGenerator) Generate(_ context.Context, sessionID string) (domainmindmap.MindMap, error) {
	g.sessionID = sessionID
	return g.mindMap, g.err
}

func (g *fakeMindMapGenerator) Regenerate(_ context.Context, id string) (domainmindmap.MindMap, error) {
	g.regenerated = id
	return g.mindMap, g.err
}

type blockingMindMapGenerator struct{ started chan struct{} }

func (g *blockingMindMapGenerator) Generate(ctx context.Context, _ string) (domainmindmap.MindMap, error) {
	close(g.started)
	<-ctx.Done()
	return domainmindmap.MindMap{}, fmt.Errorf("generating: %w", ctx.Err())
}

func (g *blockingMindMapGenerator) Regenerate(ctx context.Context, _ string) (domainmindmap.MindMap, error) {
	close(g.started)
	<-ctx.Done()
	return domainmindmap.MindMap{}, fmt.Errorf("regenerating: %w", ctx.Err())
}

type fakeMindMaps struct {
	summaries  []domainmindmap.Summary
	mindMap    domainmindmap.MindMap
	err        error
	sessionID  string
	savedID    string
	savedXML   string
	deletedID  string
	gotID      string
	listCalled bool
}

func (m *fakeMindMaps) ListMindMaps(context.Context) ([]domainmindmap.Summary, error) {
	m.listCalled = true
	return m.summaries, m.err
}

func (m *fakeMindMaps) ListSessionMindMaps(_ context.Context, sessionID string) ([]domainmindmap.Summary, error) {
	m.sessionID = sessionID
	return m.summaries, m.err
}

func (m *fakeMindMaps) GetMindMap(_ context.Context, id string) (domainmindmap.MindMap, error) {
	m.gotID = id
	return m.mindMap, m.err
}

func (m *fakeMindMaps) SaveDiagram(_ context.Context, id, diagramXML string) (domainmindmap.MindMap, error) {
	m.savedID, m.savedXML = id, diagramXML
	return m.mindMap, m.err
}

func (m *fakeMindMaps) DeleteMindMap(_ context.Context, id string) error {
	m.deletedID = id
	return m.err
}

// newMindMapTestApp builds an App with only the given options wired, so a change
// to NewApp's parameters breaks one place, not every test.
func newMindMapTestApp(options ...Option) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, options...)
	app.ctx = context.Background()
	return app
}

func newMindMapApp(generator mindMapGenerator, mindMaps mindMapUseCases) *App {
	app := newMindMapTestApp(WithMindMapGenerationService(generator), WithMindMapService(mindMaps))
	app.ctx = context.Background()
	return app
}

func TestApp_GenerateMindMap_returnsTheSummaryOfTheNewMap(t *testing.T) {
	// Given a generator that creates a mind map
	generator := &fakeMindMapGenerator{mindMap: boundMindMap()}
	app := newMindMapApp(generator, nil)

	// When the frontend generates one for a session
	got, err := app.GenerateMindMap(" session-1 ")

	// Then the summary is returned and the use case got the trimmed id
	require.NoError(t, err)
	assert.Equal(t, MindMapSummaryDTO{
		ID: "map-1", SessionID: "session-1", SessionTitle: "Caching", Title: "Caching basics",
		CreatedAt: "2026-11-17T09:30:00Z", UpdatedAt: "2026-11-17T10:30:00Z", MessageID: "message-1",
	}, got)
	assert.Equal(t, "session-1", generator.sessionID)
}

func TestApp_RegenerateMindMap_returnsTheMapWithItsDiagram(t *testing.T) {
	// Given a generator that regenerates a mind map
	generator := &fakeMindMapGenerator{mindMap: boundMindMap()}
	app := newMindMapApp(generator, nil)

	// When the frontend regenerates it
	got, err := app.RegenerateMindMap(" map-1 ")

	// Then the full map comes back and the use case got the trimmed id
	require.NoError(t, err)
	assert.Equal(t, toMindMapDTO(boundMindMap()), got)
	assert.Equal(t, "<mxGraphModel/>", got.DiagramXML)
	assert.Equal(t, "map-1", generator.regenerated)
}

func TestApp_mindMapGeneration_mapsUseCaseErrorsToCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"unknown session", applicationmindmap.ErrSourceSessionNotFound, mindMapCodeSessionNotFound},
		{"unknown map", fmt.Errorf("w: %w", domainmindmap.ErrMindMapNotFound), mindMapCodeNotFound},
		{"invalid tree", fmt.Errorf("w: %w", applicationmindmap.ErrInvalidTree), mindMapCodeInvalidGeneration},
		{"cancelled", fmt.Errorf("w: %w", context.Canceled), mindMapCodeCancelled},
		{"anything else", errors.New("provider down"), mindMapCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newMindMapApp(&fakeMindMapGenerator{err: tt.err}, nil)

			_, generateErr := app.GenerateMindMap("s")
			_, regenerateErr := app.RegenerateMindMap("m")

			for _, err := range []error{generateErr, regenerateErr} {
				assert.Equal(t, tt.code, mindMapCodeOf(t, err).Code)
			}
		})
	}
}

func TestApp_CancelMindMapGeneration_stopsTheRunningGeneration(t *testing.T) {
	// Given a generation that is still running
	generator := &blockingMindMapGenerator{started: make(chan struct{})}
	app := newMindMapApp(generator, nil)
	result := make(chan error, 1)
	go func() {
		_, err := app.GenerateMindMap("session-1")
		result <- err
	}()
	<-generator.started

	// When the frontend cancels it
	cancelErr := app.CancelMindMapGeneration()

	// Then the generation ends with the cancelled code
	require.NoError(t, cancelErr)
	assert.Equal(t, mindMapCodeCancelled, mindMapCodeOf(t, <-result).Code)
}

func TestApp_CancelMindMapGeneration_withNothingRunningDoesNothing(t *testing.T) {
	app := newMindMapApp(&fakeMindMapGenerator{}, nil)

	assert.NoError(t, app.CancelMindMapGeneration())
}

func TestApp_mindMapGeneration_refusesASecondOneWhileOneRuns(t *testing.T) {
	// Given a generation that is still running
	generator := &blockingMindMapGenerator{started: make(chan struct{})}
	app := newMindMapApp(generator, nil)
	first := make(chan error, 1)
	go func() {
		_, err := app.GenerateMindMap("session-1")
		first <- err
	}()
	<-generator.started

	// When the frontend starts a generation and a regeneration
	_, generateErr := app.GenerateMindMap("session-2")
	_, regenerateErr := app.RegenerateMindMap("map-1")

	// Then both are refused and the first one is not disturbed
	assert.Equal(t, mindMapCodeGenerationInProgress, mindMapCodeOf(t, generateErr).Code)
	assert.Equal(t, mindMapCodeGenerationInProgress, mindMapCodeOf(t, regenerateErr).Code)
	require.NoError(t, app.CancelMindMapGeneration())
	<-first
}

func TestApp_GenerateMindMap_canRunAgainAfterOneFinished(t *testing.T) {
	// Given a generation that failed
	generator := &fakeMindMapGenerator{err: errors.New("provider down")}
	app := newMindMapApp(generator, nil)
	_, err := app.GenerateMindMap("session-1")
	require.Error(t, err)

	// When the frontend retries
	generator.err = nil
	generator.mindMap = boundMindMap()
	_, retryErr := app.GenerateMindMap("session-1")

	// Then the slot was freed
	assert.NoError(t, retryErr)
}

func TestApp_ListMindMaps_returnsSummaries(t *testing.T) {
	// Given two stored maps
	mindMaps := &fakeMindMaps{summaries: []domainmindmap.Summary{
		{ID: "a", SessionID: "s1", SessionTitle: "Go", Title: "Go map", CreatedAt: mindMapTime, UpdatedAt: mindMapTime, MessageID: "m1"},
		{ID: "b", CreatedAt: mindMapTime, UpdatedAt: mindMapTime},
	}}
	app := newMindMapApp(nil, mindMaps)

	// When the frontend lists them
	got, err := app.ListMindMaps()

	// Then every field is carried over, times as RFC 3339
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, MindMapSummaryDTO{
		ID: "a", SessionID: "s1", SessionTitle: "Go", Title: "Go map",
		CreatedAt: "2026-11-17T09:30:00Z", UpdatedAt: "2026-11-17T09:30:00Z", MessageID: "m1",
	}, got[0])
	assert.Equal(t, "b", got[1].ID)
}

func TestApp_ListMindMaps_noMapsIsAnEmptyListNotNull(t *testing.T) {
	app := newMindMapApp(nil, &fakeMindMaps{})

	got, err := app.ListMindMaps()

	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestApp_ListSessionMindMaps_returnsTheCardsOfTheSession(t *testing.T) {
	// Given a session with a map from an offer
	mindMaps := &fakeMindMaps{summaries: []domainmindmap.Summary{{ID: "a", SessionID: "s1", MessageID: "m1", CreatedAt: mindMapTime, UpdatedAt: mindMapTime}}}
	app := newMindMapApp(nil, mindMaps)

	// When the frontend lists the session's maps
	got, err := app.ListSessionMindMaps(" s1 ")

	// Then the cards come back and the use case got the trimmed id
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "m1", got[0].MessageID)
	assert.Equal(t, "s1", mindMaps.sessionID)
}

func TestApp_ListSessionMindMaps_noMapsIsAnEmptyListNotNull(t *testing.T) {
	app := newMindMapApp(nil, &fakeMindMaps{})

	got, err := app.ListSessionMindMaps("s1")

	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestApp_ListSessionMindMaps_requiresASession(t *testing.T) {
	app := newMindMapApp(nil, &fakeMindMaps{})

	_, err := app.ListSessionMindMaps("   ")

	assert.Equal(t, mindMapCodeInvalidInput, mindMapCodeOf(t, err).Code)
}

func TestApp_GetMindMap_returnsTheMapWithItsDiagram(t *testing.T) {
	// Given a stored map
	mindMaps := &fakeMindMaps{mindMap: boundMindMap()}
	app := newMindMapApp(nil, mindMaps)

	// When the frontend reads it
	got, err := app.GetMindMap("map-1")

	// Then the diagram comes with the summary fields
	require.NoError(t, err)
	assert.Equal(t, toMindMapDTO(boundMindMap()), got)
	assert.Equal(t, "map-1", mindMaps.gotID)
}

func TestApp_SaveMindMapDiagram_passesTheEditAndReturnsTheMap(t *testing.T) {
	// Given a stored map
	mindMaps := &fakeMindMaps{mindMap: boundMindMap()}
	app := newMindMapApp(nil, mindMaps)

	// When the frontend saves an edited diagram
	got, err := app.SaveMindMapDiagram("map-1", "<mxGraphModel>edit</mxGraphModel>")

	// Then the use case got the id and the XML, and the saved map comes back
	require.NoError(t, err)
	assert.Equal(t, "map-1", mindMaps.savedID)
	assert.Equal(t, "<mxGraphModel>edit</mxGraphModel>", mindMaps.savedXML)
	assert.Equal(t, toMindMapDTO(boundMindMap()), got)
}

func TestApp_DeleteMindMap_passesTheID(t *testing.T) {
	mindMaps := &fakeMindMaps{}
	app := newMindMapApp(nil, mindMaps)

	require.NoError(t, app.DeleteMindMap("map-1"))

	assert.Equal(t, "map-1", mindMaps.deletedID)
}

func TestApp_mindMapBindings_mapUseCaseErrorsToCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"unknown map", fmt.Errorf("w: %w", domainmindmap.ErrMindMapNotFound), mindMapCodeNotFound},
		{"diagram too large", domainmindmap.ErrDiagramTooLarge, mindMapCodeDiagramTooLarge},
		{"diagram is not draw.io", applicationmindmap.ErrInvalidDiagram, mindMapCodeInvalidDiagram},
		{"empty diagram", domainmindmap.ErrDiagramRequired, mindMapCodeInvalidDiagram},
		{"anything else", errors.New("disk on fire"), mindMapCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newMindMapApp(nil, &fakeMindMaps{err: tt.err})

			_, listErr := app.ListMindMaps()
			_, sessionErr := app.ListSessionMindMaps("s")
			_, getErr := app.GetMindMap("m")
			_, saveErr := app.SaveMindMapDiagram("m", "x")
			deleteErr := app.DeleteMindMap("m")

			for _, err := range []error{listErr, sessionErr, getErr, saveErr, deleteErr} {
				assert.Equal(t, tt.code, mindMapCodeOf(t, err).Code)
			}
		})
	}
}

func TestMindMapError_errorIsJSON(t *testing.T) {
	err := &MindMapError{Code: "x", Message: "y"}

	assert.JSONEq(t, `{"code":"x","message":"y"}`, err.Error())
}

func TestApp_mindMapBindings_reportUnavailableWhenTheirServiceIsNotWired(t *testing.T) {
	// Given an app composed without any mind map service
	app := newMindMapTestApp()

	calls := map[string]func() error{
		"GenerateMindMap":     func() error { _, err := app.GenerateMindMap("s"); return err },
		"RegenerateMindMap":   func() error { _, err := app.RegenerateMindMap("m"); return err },
		"ListMindMaps":        func() error { _, err := app.ListMindMaps(); return err },
		"ListSessionMindMaps": func() error { _, err := app.ListSessionMindMaps("s"); return err },
		"GetMindMap":          func() error { _, err := app.GetMindMap("m"); return err },
		"SaveMindMapDiagram":  func() error { _, err := app.SaveMindMapDiagram("m", "x"); return err },
		"DeleteMindMap":       func() error { return app.DeleteMindMap("m") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// When calling the binding, Then it answers with the unavailable code instead of panicking
			var mindMapErr *MindMapError
			require.ErrorAs(t, call(), &mindMapErr)
			assert.Equal(t, mindMapCodeUnavailable, mindMapErr.Code)
		})
	}
}

func TestApp_mindMapBindings_trimTheID(t *testing.T) {
	mindMaps := &fakeMindMaps{mindMap: boundMindMap()}
	app := newMindMapApp(&fakeMindMapGenerator{}, mindMaps)

	_, err := app.GetMindMap("  map-1 ")
	require.NoError(t, err)
	assert.Equal(t, "map-1", mindMaps.gotID)

	_, err = app.SaveMindMapDiagram(" map-1\n", "<mxGraphModel/>")
	require.NoError(t, err)
	assert.Equal(t, "map-1", mindMaps.savedID)

	require.NoError(t, app.DeleteMindMap("\tmap-1 "))
	assert.Equal(t, "map-1", mindMaps.deletedID)
}

func TestApp_mindMapBindings_requireAnID(t *testing.T) {
	mindMaps := &fakeMindMaps{mindMap: boundMindMap()}
	app := newMindMapApp(&fakeMindMapGenerator{}, mindMaps)

	_, getErr := app.GetMindMap("  ")
	_, saveErr := app.SaveMindMapDiagram("", "<mxGraphModel/>")
	deleteErr := app.DeleteMindMap(" ")

	for _, err := range []error{getErr, saveErr, deleteErr} {
		assert.Equal(t, mindMapCodeInvalidInput, mindMapCodeOf(t, err).Code)
	}
	assert.Empty(t, mindMaps.gotID)
	assert.Empty(t, mindMaps.savedID)
	assert.Empty(t, mindMaps.deletedID)
}

func TestToMindMapError_mapsADeadlineToTimeout(t *testing.T) {
	err := toMindMapError(fmt.Errorf("generating: %w", context.DeadlineExceeded))

	assert.Equal(t, mindMapCodeTimeout, err.Code)
}
