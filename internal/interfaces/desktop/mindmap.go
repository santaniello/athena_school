package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	applicationmindmap "github.com/santaniello/athena/internal/application/mindmap"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

// mindMapGenerator is the use case behind GenerateMindMap and RegenerateMindMap
// (spec 3.24).
type mindMapGenerator interface {
	Generate(ctx context.Context, sessionID string) (domainmindmap.MindMap, error)
	Regenerate(ctx context.Context, id string) (domainmindmap.MindMap, error)
}

// WithMindMapGenerationService wires mind map generation from a studied session.
func WithMindMapGenerationService(generator mindMapGenerator) Option {
	return func(a *App) { a.mindMapGenerator = generator }
}

// mindMapUseCases is the use cases behind the catalog, edit and delete bindings
// of the mind maps (spec 3.24).
type mindMapUseCases interface {
	ListMindMaps(ctx context.Context) ([]domainmindmap.Summary, error)
	ListSessionMindMaps(ctx context.Context, sessionID string) ([]domainmindmap.Summary, error)
	GetMindMap(ctx context.Context, id string) (domainmindmap.MindMap, error)
	SaveDiagram(ctx context.Context, id, diagramXML string) (domainmindmap.MindMap, error)
	DeleteMindMap(ctx context.Context, id string) error
}

// WithMindMapService wires the mind map catalog, edit and delete.
func WithMindMapService(mindMaps mindMapUseCases) Option {
	return func(a *App) { a.mindMaps = mindMaps }
}

// Stable MindMapError.Code values the frontend switches on.
const (
	mindMapCodeInvalidInput         = "invalid_input"
	mindMapCodeSessionNotFound      = "source_session_not_found"
	mindMapCodeNotFound             = "mind_map_not_found"
	mindMapCodeInvalidGeneration    = "invalid_generation"
	mindMapCodeInvalidDiagram       = "invalid_diagram"
	mindMapCodeDiagramTooLarge      = "diagram_too_large"
	mindMapCodeGenerationInProgress = "generation_in_progress"
	mindMapCodeCancelled            = "cancelled"
	mindMapCodeTimeout              = "timeout"
	mindMapCodeUnavailable          = "unavailable"
	mindMapCodeInternal             = "internal"
)

// MindMapError is the error the mind map bindings return. Wails hands the
// frontend only the error's text, so Error() is this struct as JSON:
// {"code": "...", "message": "..."}.
type MindMapError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *MindMapError) Error() string {
	encoded, err := json.Marshal(e)
	if err != nil {
		return e.Code
	}
	return string(encoded)
}

// toMindMapError maps a use case error to its coded form. Anything unknown is
// logged with its detail and reported as a generic internal error.
func toMindMapError(err error) *MindMapError {
	switch {
	case errors.Is(err, applicationmindmap.ErrSourceSessionNotFound):
		return &MindMapError{Code: mindMapCodeSessionNotFound, Message: "study session not found"}
	case errors.Is(err, domainmindmap.ErrMindMapNotFound):
		return &MindMapError{Code: mindMapCodeNotFound, Message: "mind map not found"}
	case errors.Is(err, applicationmindmap.ErrInvalidTree):
		return &MindMapError{Code: mindMapCodeInvalidGeneration, Message: "the generated mind map was not valid, try again"}
	case errors.Is(err, domainmindmap.ErrDiagramTooLarge):
		return &MindMapError{Code: mindMapCodeDiagramTooLarge, Message: "the diagram is too large to save"}
	case errors.Is(err, applicationmindmap.ErrInvalidDiagram), errors.Is(err, domainmindmap.ErrDiagramRequired):
		return &MindMapError{Code: mindMapCodeInvalidDiagram, Message: "the diagram is not valid"}
	case errors.Is(err, context.Canceled):
		return &MindMapError{Code: mindMapCodeCancelled, Message: "cancelled"}
	case errors.Is(err, context.DeadlineExceeded):
		return &MindMapError{Code: mindMapCodeTimeout, Message: "the request took too long, try again"}
	}
	log.Printf("mind map binding failed: %v", err)
	return &MindMapError{Code: mindMapCodeInternal, Message: "something went wrong"}
}

// mindMapUnavailable is the error of a mind map binding whose service was not
// wired.
func mindMapUnavailable() *MindMapError {
	return &MindMapError{Code: mindMapCodeUnavailable, Message: "mind maps are not available"}
}

// MindMapSummaryDTO is a mind map as the catalog and the chat cards list it.
// CreatedAt and UpdatedAt are RFC 3339. MessageID is the study-chat message
// whose offer produced the map, empty for a map from the menu.
type MindMapSummaryDTO struct {
	ID           string `json:"id"`
	SessionID    string `json:"sessionId"`
	SessionTitle string `json:"sessionTitle"`
	Title        string `json:"title"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	MessageID    string `json:"messageId"`
}

// MindMapDTO is a mind map to show and edit: the draw.io document included.
type MindMapDTO struct {
	MindMapSummaryDTO
	DiagramXML string `json:"diagramXml"`
}

// toMindMapSummaryDTO converts a mind map to its catalog form. A Summary goes
// through it too, so the DTO's fields are mapped in one place.
func toMindMapSummaryDTO(m domainmindmap.MindMap) MindMapSummaryDTO {
	return MindMapSummaryDTO{
		ID: m.ID, SessionID: m.SessionID, SessionTitle: m.SessionTitle, Title: m.Title,
		CreatedAt: m.CreatedAt.Format(time.RFC3339), UpdatedAt: m.UpdatedAt.Format(time.RFC3339), MessageID: m.MessageID,
	}
}

func toMindMapDTO(m domainmindmap.MindMap) MindMapDTO {
	return MindMapDTO{MindMapSummaryDTO: toMindMapSummaryDTO(m), DiagramXML: m.DiagramXML}
}

func toMindMapSummaryDTOs(summaries []domainmindmap.Summary) []MindMapSummaryDTO {
	dtos := make([]MindMapSummaryDTO, 0, len(summaries))
	for _, summary := range summaries {
		dtos = append(dtos, toMindMapSummaryDTO(domainmindmap.MindMap{
			ID: summary.ID, SessionID: summary.SessionID, SessionTitle: summary.SessionTitle, Title: summary.Title,
			CreatedAt: summary.CreatedAt, UpdatedAt: summary.UpdatedAt, MessageID: summary.MessageID,
		}))
	}
	return dtos
}

// beginMindMapGeneration reserves the generation slot shared by GenerateMindMap
// and RegenerateMindMap. It returns the context the generation runs under and
// the function that frees the slot, or the error to return when the service is
// not wired or a generation already runs.
func (a *App) beginMindMapGeneration() (context.Context, func(), *MindMapError) {
	if a.mindMapGenerator == nil {
		return nil, nil, mindMapUnavailable()
	}
	ctx, release, ok := a.mindMapGeneration.begin(a.ctx)
	if !ok {
		return nil, nil, &MindMapError{Code: mindMapCodeGenerationInProgress, Message: "a mind map is already being generated"}
	}
	return ctx, release, nil
}

// requireMindMapID trims a mind map id and refuses an empty one.
func requireMindMapID(id string) (string, *MindMapError) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", &MindMapError{Code: mindMapCodeInvalidInput, Message: "mind map id is required"}
	}
	return id, nil
}

// GenerateMindMap asks for a mind map about a studied session. Only one
// generation runs at a time, regenerations included.
func (a *App) GenerateMindMap(sessionID string) (MindMapSummaryDTO, error) {
	ctx, release, begun := a.beginMindMapGeneration()
	if begun != nil {
		return MindMapSummaryDTO{}, begun
	}
	defer release()
	mindMap, err := a.mindMapGenerator.Generate(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return MindMapSummaryDTO{}, toMindMapError(err)
	}
	return toMindMapSummaryDTO(mindMap), nil
}

// RegenerateMindMap draws a mind map again from its session's current content,
// replacing what the user edited by hand. A failure leaves the map untouched.
// It shares the generation slot with GenerateMindMap.
func (a *App) RegenerateMindMap(id string) (MindMapDTO, error) {
	ctx, release, begun := a.beginMindMapGeneration()
	if begun != nil {
		return MindMapDTO{}, begun
	}
	defer release()
	mindMap, err := a.mindMapGenerator.Regenerate(ctx, strings.TrimSpace(id))
	if err != nil {
		return MindMapDTO{}, toMindMapError(err)
	}
	return toMindMapDTO(mindMap), nil
}

// CancelMindMapGeneration cancels the mind map generation or regeneration in
// flight, which then fails with the cancelled code and stores nothing. With
// none running it does nothing.
func (a *App) CancelMindMapGeneration() error {
	a.mindMapGeneration.cancel()
	return nil
}

// ListMindMaps returns every mind map, newest first, without its diagram.
func (a *App) ListMindMaps() ([]MindMapSummaryDTO, error) {
	if a.mindMaps == nil {
		return nil, mindMapUnavailable()
	}
	summaries, err := a.mindMaps.ListMindMaps(a.ctx)
	if err != nil {
		return nil, toMindMapError(err)
	}
	return toMindMapSummaryDTOs(summaries), nil
}

// ListSessionMindMaps returns the mind maps the study chat's offers produced in
// the session, oldest first, without their diagrams: the cards the chat shows
// under the messages that offered them.
func (a *App) ListSessionMindMaps(sessionID string) ([]MindMapSummaryDTO, error) {
	if a.mindMaps == nil {
		return nil, mindMapUnavailable()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, &MindMapError{Code: mindMapCodeInvalidInput, Message: "session id is required"}
	}
	summaries, err := a.mindMaps.ListSessionMindMaps(a.ctx, sessionID)
	if err != nil {
		return nil, toMindMapError(err)
	}
	return toMindMapSummaryDTOs(summaries), nil
}

// GetMindMap returns a mind map with its draw.io document.
func (a *App) GetMindMap(id string) (MindMapDTO, error) {
	if a.mindMaps == nil {
		return MindMapDTO{}, mindMapUnavailable()
	}
	id, invalid := requireMindMapID(id)
	if invalid != nil {
		return MindMapDTO{}, invalid
	}
	mindMap, err := a.mindMaps.GetMindMap(a.ctx, id)
	if err != nil {
		return MindMapDTO{}, toMindMapError(err)
	}
	return toMindMapDTO(mindMap), nil
}

// SaveMindMapDiagram stores the user's edit of a mind map's draw.io document.
func (a *App) SaveMindMapDiagram(id, diagramXML string) (MindMapDTO, error) {
	if a.mindMaps == nil {
		return MindMapDTO{}, mindMapUnavailable()
	}
	id, invalid := requireMindMapID(id)
	if invalid != nil {
		return MindMapDTO{}, invalid
	}
	mindMap, err := a.mindMaps.SaveDiagram(a.ctx, id, diagramXML)
	if err != nil {
		return MindMapDTO{}, toMindMapError(err)
	}
	return toMindMapDTO(mindMap), nil
}

// DeleteMindMap removes a mind map.
func (a *App) DeleteMindMap(id string) error {
	if a.mindMaps == nil {
		return mindMapUnavailable()
	}
	id, invalid := requireMindMapID(id)
	if invalid != nil {
		return invalid
	}
	if err := a.mindMaps.DeleteMindMap(a.ctx, id); err != nil {
		return toMindMapError(err)
	}
	return nil
}
