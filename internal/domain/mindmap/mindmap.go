// Package mindmap is the bounded context of mind maps: visual summaries of a
// study session that the user can open, edit, regenerate and delete. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
package mindmap

import (
	"errors"
	"strings"
	"time"
)

// MaxDiagramBytes caps the stored draw.io XML, as it caps a lab submission.
const MaxDiagramBytes = 2 << 20

var (
	// ErrSessionRequired is returned when a mind map has no source session.
	ErrSessionRequired = errors.New("mind map session is required")
	// ErrSessionTitleRequired is returned when a mind map has no session title snapshot.
	ErrSessionTitleRequired = errors.New("mind map session title is required")
	// ErrTitleRequired is returned when a mind map has no title.
	ErrTitleRequired = errors.New("mind map title is required")
	// ErrTreeRequired is returned when a mind map has no tree.
	ErrTreeRequired = errors.New("mind map tree is required")
	// ErrDiagramRequired is returned when a mind map has no diagram.
	ErrDiagramRequired = errors.New("mind map diagram is required")
	// ErrDiagramTooLarge is returned when the diagram is over MaxDiagramBytes.
	ErrDiagramTooLarge = errors.New("mind map diagram is too large")
)

// MindMap is a mind map generated from one study session. Tree is the JSON the
// model produced, kept so the map can be shown again without a new call;
// DiagramXML is the draw.io document the user sees and edits. Deleting the
// session deletes its mind maps.
type MindMap struct {
	ID           string
	SessionID    string
	SessionTitle string
	Title        string
	Tree         string
	DiagramXML   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// MessageID is the study-chat message whose offer produced the map. It is
	// empty for a map generated from the menu.
	MessageID string
}

// Validate reports the first rule the mind map breaks.
func (m MindMap) Validate() error {
	if strings.TrimSpace(m.SessionID) == "" {
		return ErrSessionRequired
	}
	if strings.TrimSpace(m.SessionTitle) == "" {
		return ErrSessionTitleRequired
	}
	if strings.TrimSpace(m.Title) == "" {
		return ErrTitleRequired
	}
	if strings.TrimSpace(m.Tree) == "" {
		return ErrTreeRequired
	}
	if strings.TrimSpace(m.DiagramXML) == "" {
		return ErrDiagramRequired
	}
	if len(m.DiagramXML) > MaxDiagramBytes {
		return ErrDiagramTooLarge
	}
	return nil
}
