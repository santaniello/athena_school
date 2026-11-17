package mindmap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

// GenerationService writes a mind map from a studied session with one model
// call. See specs/phases/phase-03-architecture-lab/24-mind-map.md.
type GenerationService struct {
	sessions SessionReader
	contexts generationContextBuilder
	llm      domainllm.Provider
	repo     domainmindmap.Repository
	renderer Renderer
	now      func() time.Time
	newID    func() string
}

// NewGenerationService creates a GenerationService backed by the given ports.
func NewGenerationService(
	sessions SessionReader,
	summaries SummaryReader,
	profiles ProfileReader,
	notes NotesSearcher,
	llm domainllm.Provider,
	repo domainmindmap.Repository,
) *GenerationService {
	return &GenerationService{
		sessions: sessions,
		contexts: generationContextBuilder{summaries: summaries, profiles: profiles, notes: notes},
		llm:      llm,
		repo:     repo,
		now:      func() time.Time { return time.Now().UTC() },
		newID:    uuid.NewString,
	}
}

// Generate asks the model for a mind map of the session, renders it and saves
// it. It makes exactly one model call and never retries: on any failure nothing
// is stored and the caller may call again. It returns ErrSourceSessionNotFound
// for an unknown session and ErrInvalidTree when the model's answer breaks a
// tree rule.
func (s *GenerationService) Generate(ctx context.Context, sessionID string) (domainmindmap.MindMap, error) {
	return s.generate(ctx, sessionID, "")
}

// GenerateForMessage is Generate for a mind map an offer in the study chat asked
// for: the stored map carries the id of the message the offer hangs under.
func (s *GenerationService) GenerateForMessage(ctx context.Context, sessionID, messageID string) (domainmindmap.MindMap, error) {
	return s.generate(ctx, sessionID, messageID)
}

// Regenerate draws the mind map again from the session's current content and
// replaces its title, tree and diagram, keeping its id, session, message and
// creation time. Edits the user made by hand are lost. A failure leaves the
// stored map untouched. It returns domainmindmap.ErrMindMapNotFound for an
// unknown map.
func (s *GenerationService) Regenerate(ctx context.Context, id string) (domainmindmap.MindMap, error) {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("mindmap: reading mind map: %w", err)
	}
	drawn, err := s.draw(ctx, current.SessionID)
	if err != nil {
		return domainmindmap.MindMap{}, err
	}
	current.SessionTitle = drawn.sessionTitle
	current.Title = drawn.title
	current.Tree = drawn.tree
	current.DiagramXML = drawn.diagramXML
	current.UpdatedAt = s.now()
	if err := current.Validate(); err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("%w: %w", ErrInvalidTree, err)
	}
	if err := s.repo.Update(ctx, current); err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("mindmap: saving regenerated mind map: %w", err)
	}
	return current, nil
}

func (s *GenerationService) generate(ctx context.Context, sessionID, messageID string) (domainmindmap.MindMap, error) {
	drawn, err := s.draw(ctx, sessionID)
	if err != nil {
		return domainmindmap.MindMap{}, err
	}
	now := s.now()
	mindMap := domainmindmap.MindMap{
		ID:           s.newID(),
		SessionID:    strings.TrimSpace(sessionID),
		SessionTitle: drawn.sessionTitle,
		Title:        drawn.title,
		Tree:         drawn.tree,
		DiagramXML:   drawn.diagramXML,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageID:    messageID,
	}
	if err := mindMap.Validate(); err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("%w: %w", ErrInvalidTree, err)
	}
	if err := s.repo.Create(ctx, mindMap); err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("mindmap: saving generated mind map: %w", err)
	}
	return mindMap, nil
}

// drawing is what one model call produces for a session.
type drawing struct {
	sessionTitle string
	title        string
	tree         string
	diagramXML   string
}

// draw reads the session, asks the model for the tree and renders it. It stores
// nothing.
func (s *GenerationService) draw(ctx context.Context, sessionID string) (drawing, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return drawing{}, ErrSourceSessionNotFound
	}
	session, err := s.sessions.ReadSession(ctx, sessionID)
	if err != nil {
		return drawing{}, fmt.Errorf("mindmap: reading source session: %w", err)
	}
	gc, err := s.contexts.build(ctx, sessionID, session)
	if err != nil {
		return drawing{}, err
	}
	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      generationTask,
		Messages:  buildPrompt(gc),
	})
	if err != nil {
		return drawing{}, fmt.Errorf("mindmap: generating mind map: %w", err)
	}
	raw := stripCodeFence(response.Content)
	root, err := ParseTree(raw)
	if err != nil {
		return drawing{}, err
	}
	return drawing{
		sessionTitle: strings.TrimSpace(session.Topic),
		title:        CleanLabel(root.Label),
		tree:         string(raw),
		diagramXML:   s.renderer.RenderTree(root),
	}, nil
}
