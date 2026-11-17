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

// mindMapOffers is the use case behind the mind map offer bindings of the study
// chat (spec 3.24).
type mindMapOffers interface {
	GetPending(ctx context.Context, sessionID string) (domainmindmap.Offer, bool, error)
	Accept(ctx context.Context, offerID string) (domainmindmap.Summary, error)
	Decline(ctx context.Context, offerID string) error
}

// WithMindMapOfferService wires the mind map offers of the study chat.
func WithMindMapOfferService(offers mindMapOffers) Option {
	return func(a *App) { a.mindMapOffers = offers }
}

// Stable MindMapError.Code values the frontend switches on.
const (
	mindMapCodeInvalidInput         = "invalid_input"
	mindMapCodeSessionNotFound      = "source_session_not_found"
	mindMapCodeNotFound             = "mind_map_not_found"
	mindMapCodeOfferNotFound        = "offer_not_found"
	mindMapCodeOfferPending         = "offer_pending"
	mindMapCodeInvalidGeneration    = "invalid_generation"
	mindMapCodeInvalidDiagram       = "invalid_diagram"
	mindMapCodeDiagramTooLarge      = "diagram_too_large"
	mindMapCodeGenerationInProgress = "generation_in_progress"
	mindMapCodeCancelled            = "cancelled"
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
	case errors.Is(err, domainmindmap.ErrOfferNotFound):
		return &MindMapError{Code: mindMapCodeOfferNotFound, Message: "the mind map offer is no longer pending"}
	case errors.Is(err, domainmindmap.ErrOfferPending):
		return &MindMapError{Code: mindMapCodeOfferPending, Message: "a mind map offer is already pending"}
	case errors.Is(err, applicationmindmap.ErrInvalidTree):
		return &MindMapError{Code: mindMapCodeInvalidGeneration, Message: "the generated mind map was not valid, try again"}
	case errors.Is(err, domainmindmap.ErrDiagramTooLarge):
		return &MindMapError{Code: mindMapCodeDiagramTooLarge, Message: "the diagram is too large to save"}
	case errors.Is(err, applicationmindmap.ErrInvalidDiagram), errors.Is(err, domainmindmap.ErrDiagramRequired):
		return &MindMapError{Code: mindMapCodeInvalidDiagram, Message: "the diagram is not valid"}
	case errors.Is(err, context.Canceled):
		return &MindMapError{Code: mindMapCodeCancelled, Message: "cancelled"}
	}
	log.Printf("mind map binding failed: %v", err)
	return &MindMapError{Code: mindMapCodeInternal, Message: "something went wrong"}
}

// mindMapUnavailable is the error of a mind map binding whose service was not
// wired.
func mindMapUnavailable() *MindMapError {
	return &MindMapError{Code: mindMapCodeUnavailable, Message: "mind maps are not available"}
}

// MindMapOfferDTO is a pending mind map offer: it hangs under the assistant
// message MessageID of the study chat.
type MindMapOfferDTO struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"`
}

func mindMapInvalidInput(message string) *MindMapError {
	return &MindMapError{Code: mindMapCodeInvalidInput, Message: message}
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
		dtos = append(dtos, MindMapSummaryDTO{
			ID: summary.ID, SessionID: summary.SessionID, SessionTitle: summary.SessionTitle, Title: summary.Title,
			CreatedAt: summary.CreatedAt.Format(time.RFC3339), UpdatedAt: summary.UpdatedAt.Format(time.RFC3339),
			MessageID: summary.MessageID,
		})
	}
	return dtos
}

// GenerateMindMap asks for a mind map about a studied session. Only one
// generation runs at a time, regenerations included.
func (a *App) GenerateMindMap(sessionID string) (MindMapSummaryDTO, error) {
	if a.mindMapGenerator == nil {
		return MindMapSummaryDTO{}, mindMapUnavailable()
	}
	ctx, release, ok := a.mindMapGeneration.begin(a.ctx)
	if !ok {
		return MindMapSummaryDTO{}, &MindMapError{Code: mindMapCodeGenerationInProgress, Message: "a mind map is already being generated"}
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
	if a.mindMapGenerator == nil {
		return MindMapDTO{}, mindMapUnavailable()
	}
	ctx, release, ok := a.mindMapGeneration.begin(a.ctx)
	if !ok {
		return MindMapDTO{}, &MindMapError{Code: mindMapCodeGenerationInProgress, Message: "a mind map is already being generated"}
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
		return nil, mindMapInvalidInput("session id is required")
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
	if err := a.mindMaps.DeleteMindMap(a.ctx, id); err != nil {
		return toMindMapError(err)
	}
	return nil
}

// GetPendingMindMapOffer returns the session's pending mind map offer, or null
// when there is none.
func (a *App) GetPendingMindMapOffer(sessionID string) (*MindMapOfferDTO, error) {
	if a.mindMapOffers == nil {
		return nil, mindMapUnavailable()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, mindMapInvalidInput("session id is required")
	}
	offer, found, err := a.mindMapOffers.GetPending(a.ctx, sessionID)
	if err != nil {
		return nil, toMindMapError(err)
	}
	if !found {
		return nil, nil
	}
	return &MindMapOfferDTO{ID: offer.ID, SessionID: offer.SessionID, MessageID: offer.MessageID}, nil
}

// AcceptMindMapOffer generates a mind map for the offer's session, links it to
// the offer's message and returns its card. It shares the generation slot with
// GenerateMindMap, so CancelMindMapGeneration cancels it. On failure the offer
// stays pending.
func (a *App) AcceptMindMapOffer(offerID string) (MindMapSummaryDTO, error) {
	if a.mindMapOffers == nil {
		return MindMapSummaryDTO{}, mindMapUnavailable()
	}
	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return MindMapSummaryDTO{}, mindMapInvalidInput("offer id is required")
	}
	ctx, release, ok := a.mindMapGeneration.begin(a.ctx)
	if !ok {
		return MindMapSummaryDTO{}, &MindMapError{Code: mindMapCodeGenerationInProgress, Message: "a mind map is already being generated"}
	}
	defer release()
	summary, err := a.mindMapOffers.Accept(ctx, offerID)
	if err != nil {
		return MindMapSummaryDTO{}, toMindMapError(err)
	}
	return toMindMapSummaryDTOs([]domainmindmap.Summary{summary})[0], nil
}

// DeclineMindMapOffer removes the offer. Nothing is created.
func (a *App) DeclineMindMapOffer(offerID string) error {
	if a.mindMapOffers == nil {
		return mindMapUnavailable()
	}
	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return mindMapInvalidInput("offer id is required")
	}
	if err := a.mindMapOffers.Decline(a.ctx, offerID); err != nil {
		return toMindMapError(err)
	}
	return nil
}
