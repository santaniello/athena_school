package mindmap

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validMindMap() MindMap {
	return MindMap{
		ID:           "map-1",
		SessionID:    "session-1",
		SessionTitle: "Caching",
		Title:        "Caching",
		Tree:         `{"label":"Caching","children":[{"label":"Eviction"}]}`,
		DiagramXML:   "<mxGraphModel/>",
		CreatedAt:    time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC),
	}
}

func TestMindMapValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*MindMap)
		want   error
	}{
		{"valid", func(*MindMap) {}, nil},
		{"valid without message", func(m *MindMap) { m.MessageID = "" }, nil},
		{"session required", func(m *MindMap) { m.SessionID = "  " }, ErrSessionRequired},
		{"session title required", func(m *MindMap) { m.SessionTitle = "" }, ErrSessionTitleRequired},
		{"title required", func(m *MindMap) { m.Title = "\t" }, ErrTitleRequired},
		{"tree required", func(m *MindMap) { m.Tree = " " }, ErrTreeRequired},
		{"diagram required", func(m *MindMap) { m.DiagramXML = "" }, ErrDiagramRequired},
		{"diagram only whitespace", func(m *MindMap) { m.DiagramXML = "  \n\t" }, ErrDiagramRequired},
		{"diagram at the limit", func(m *MindMap) { m.DiagramXML = strings.Repeat("x", MaxDiagramBytes) }, nil},
		{"diagram over the limit", func(m *MindMap) { m.DiagramXML = strings.Repeat("x", MaxDiagramBytes+1) }, ErrDiagramTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a valid mind map changed by the case
			m := validMindMap()
			tt.change(&m)

			// When it is validated
			err := m.Validate()

			// Then it reports the first rule broken, or none
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestMindMapValidateReportsSessionBeforeTitle(t *testing.T) {
	// Given a mind map missing both the session and the title
	m := validMindMap()
	m.SessionID = ""
	m.Title = ""

	// When it is validated
	err := m.Validate()

	// Then the session is the first rule reported
	assert.ErrorIs(t, err, ErrSessionRequired)
}
