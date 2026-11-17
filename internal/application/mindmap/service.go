package mindmap

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

// ErrInvalidDiagram is returned when the diagram a user saves is not a draw.io
// document.
var ErrInvalidDiagram = errors.New("the diagram is not a draw.io document")

// Service implements the mind map catalog, edit and delete use cases.
type Service struct {
	repo domainmindmap.Repository
	now  func() time.Time
}

// NewService creates a Service backed by repo.
func NewService(repo domainmindmap.Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

// ListMindMaps returns every mind map, newest first, without their diagrams.
func (s *Service) ListMindMaps(ctx context.Context) ([]domainmindmap.Summary, error) {
	summaries, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("mindmap: listing mind maps: %w", err)
	}
	return summaries, nil
}

// GetMindMap returns a mind map with its tree and diagram, or
// domainmindmap.ErrMindMapNotFound.
func (s *Service) GetMindMap(ctx context.Context, id string) (domainmindmap.MindMap, error) {
	mindMap, err := s.repo.Get(ctx, id)
	if err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("mindmap: reading mind map: %w", err)
	}
	return mindMap, nil
}

// SaveDiagram replaces the diagram of a mind map with the user's edit and
// stamps the update time; the tree stays the model's. It returns
// domainmindmap.ErrMindMapNotFound for an unknown map, ErrDiagramTooLarge for a
// document over the cap and ErrInvalidDiagram for one that is not draw.io XML.
func (s *Service) SaveDiagram(ctx context.Context, id, diagramXML string) (domainmindmap.MindMap, error) {
	mindMap, err := s.repo.Get(ctx, id)
	if err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("mindmap: reading mind map: %w", err)
	}
	mindMap.DiagramXML = diagramXML
	mindMap.UpdatedAt = s.now()
	if err := mindMap.Validate(); err != nil {
		return domainmindmap.MindMap{}, err
	}
	if !isDrawioDocument(diagramXML) {
		return domainmindmap.MindMap{}, ErrInvalidDiagram
	}
	if err := s.repo.Update(ctx, mindMap); err != nil {
		return domainmindmap.MindMap{}, fmt.Errorf("mindmap: saving diagram: %w", err)
	}
	return mindMap, nil
}

// DeleteMindMap removes a mind map, or returns domainmindmap.ErrMindMapNotFound.
func (s *Service) DeleteMindMap(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("mindmap: deleting mind map: %w", err)
	}
	return nil
}

// ListSessionMindMaps returns the mind maps the study chat's offers produced in
// the session, oldest first: what the chat cards show.
func (s *Service) ListSessionMindMaps(ctx context.Context, sessionID string) ([]domainmindmap.Summary, error) {
	summaries, err := s.repo.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("mindmap: listing session mind maps: %w", err)
	}
	return summaries, nil
}

// isDrawioDocument reports whether the text is one well-formed XML document
// whose root is a draw.io model (mxGraphModel, which must hold a <root>) or
// file (mxfile, which must hold a <diagram>). Anything after the root element
// other than whitespace, comments or processing instructions is refused.
func isDrawioDocument(text string) bool {
	dec := xml.NewDecoder(strings.NewReader(text))
	rootName := ""
	depth := 0
	hasContent := false
	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return hasContent // the decoder already refuses an unclosed element
		}
		if err != nil {
			return false
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				if rootName != "" {
					return false // a second root element
				}
				rootName = t.Name.Local
				if _, known := requiredChild[rootName]; !known {
					return false
				}
			} else if depth == 1 && t.Name.Local == requiredChild[rootName] {
				hasContent = true
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) > 0 {
				return false // text outside the root element
			}
		}
	}
}

// requiredChild is the element a draw.io root must hold to describe a diagram.
var requiredChild = map[string]string{"mxGraphModel": "root", "mxfile": "diagram"}
