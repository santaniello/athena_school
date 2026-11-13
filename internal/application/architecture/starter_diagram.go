package architecture

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
)

// ErrInvalidStarterDiagram is returned when the structure a model describes
// a starter diagram with breaks a rule, so it cannot be drawn.
var ErrInvalidStarterDiagram = errors.New("the starter diagram is invalid")

// Limits on a starter diagram's structure.
const (
	MinStarterNodes = 2
	MaxStarterNodes = 10
	MaxStarterEdges = 15
)

// Layout of the rendered diagram, in draw.io units.
const (
	nodeWidth    = 140
	nodeHeight   = 60
	columnSpread = 220
	rowSpread    = 100
)

const (
	kindClient = "client"
	kindOther  = "other"
)

// kindStyles maps each node kind to a draw.io style. It is a closed set: an
// unknown kind is drawn as "other".
var kindStyles = map[string]string{
	kindClient: "rounded=1;whiteSpace=wrap;fillColor=#dae8fc;strokeColor=#6c8ebf;",
	"service":  "rounded=1;whiteSpace=wrap;fillColor=#d5e8d4;strokeColor=#82b366;",
	"database": "shape=cylinder3;whiteSpace=wrap;boundedLbl=1;fillColor=#fff2cc;strokeColor=#d6b656;",
	"cache":    "rounded=1;whiteSpace=wrap;fillColor=#ffe6cc;strokeColor=#d79b00;",
	"queue":    "shape=parallelogram;whiteSpace=wrap;fillColor=#e1d5e7;strokeColor=#9673a6;",
	"external": "rounded=1;whiteSpace=wrap;dashed=1;fillColor=#f5f5f5;strokeColor=#666666;",
	kindOther:  "rounded=1;whiteSpace=wrap;fillColor=#ffffff;strokeColor=#000000;",
}

const edgeStyle = "edgeStyle=orthogonalEdgeStyle;rounded=1;endArrow=classic;"

// StarterDiagramRenderer turns the structure a model describes, nodes and
// edges, into a valid draw.io mxGraphModel with a simple layered layout. The
// model never writes XML: coordinates, styles and ids are ours. See
// specs/phases/phase-03-architecture-lab/19-practice-starter-diagram.md.
type StarterDiagramRenderer struct{}

type starterStructure struct {
	Nodes []starterNode `json:"nodes"`
	Edges []starterEdge `json:"edges"`
}

type starterNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type starterEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// Render reads the JSON structure and returns the mxGraphModel XML. Every
// failure wraps ErrInvalidStarterDiagram.
func (StarterDiagramRenderer) Render(raw []byte) (string, error) {
	var structure starterStructure
	if err := json.Unmarshal(raw, &structure); err != nil {
		return "", fmt.Errorf("%w: reading the structure: %w", ErrInvalidStarterDiagram, err)
	}
	index, err := validateStructure(structure)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidStarterDiagram, err)
	}
	return drawStructure(structure, index), nil
}

// validateStructure checks the limits and references and returns each node's
// position by id.
func validateStructure(s starterStructure) (map[string]int, error) {
	if len(s.Nodes) < MinStarterNodes || len(s.Nodes) > MaxStarterNodes {
		return nil, fmt.Errorf("%d nodes, want %d to %d", len(s.Nodes), MinStarterNodes, MaxStarterNodes)
	}
	if len(s.Edges) > MaxStarterEdges {
		return nil, fmt.Errorf("%d edges, want at most %d", len(s.Edges), MaxStarterEdges)
	}
	index := make(map[string]int, len(s.Nodes))
	for i, node := range s.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			return nil, fmt.Errorf("node %d has no id", i+1)
		}
		if strings.TrimSpace(node.Label) == "" {
			return nil, fmt.Errorf("node %q has no label", id)
		}
		if _, taken := index[id]; taken {
			return nil, fmt.Errorf("node id %q is repeated", id)
		}
		index[id] = i
	}
	for i, edge := range s.Edges {
		_, fromKnown := index[strings.TrimSpace(edge.From)]
		_, toKnown := index[strings.TrimSpace(edge.To)]
		if !fromKnown || !toKnown {
			return nil, fmt.Errorf("edge %d links an unknown node", i+1)
		}
	}
	return index, nil
}

func drawStructure(s starterStructure, index map[string]int) string {
	columns := columnsOf(s, index)
	rows := map[int]int{}
	var b strings.Builder
	b.WriteString(`<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/>`)
	for i, node := range s.Nodes {
		column := columns[i]
		row := rows[column]
		rows[column]++
		fmt.Fprintf(&b,
			`<mxCell id="n%d" value="%s" style="%s" vertex="1" parent="1"><mxGeometry x="%d" y="%d" width="%d" height="%d" as="geometry"/></mxCell>`,
			i, escapeLabel(node.Label), styleOf(node.Kind), column*columnSpread, row*rowSpread, nodeWidth, nodeHeight)
	}
	for i, edge := range s.Edges {
		fmt.Fprintf(&b,
			`<mxCell id="e%d" value="%s" style="%s" edge="1" parent="1" source="n%d" target="n%d"><mxGeometry relative="1" as="geometry"/></mxCell>`,
			i, escapeLabel(edge.Label), edgeStyle, index[strings.TrimSpace(edge.From)], index[strings.TrimSpace(edge.To)])
	}
	b.WriteString(`</root></mxGraphModel>`)
	return b.String()
}

func styleOf(kind string) string {
	if style, ok := kindStyles[strings.ToLower(strings.TrimSpace(kind))]; ok {
		return style
	}
	return kindStyles[kindOther]
}

// escapeLabel drops the characters XML 1.0 forbids, trims the label, bounds
// its length and escapes it for an XML attribute.
func escapeLabel(label string) string {
	runes := []rune(strings.TrimSpace(strings.Map(xmlRune, label)))
	return html.EscapeString(string(runes[:min(len(runes), MaxLabelLength)]))
}

// xmlRune keeps a rune XML 1.0 allows and drops any other (-1).
func xmlRune(r rune) rune {
	switch {
	case r == '\t', r == '\n', r == '\r',
		r >= 0x20 && r <= 0xD7FF,
		r >= 0xE000 && r <= 0xFFFD,
		r >= 0x10000 && r <= 0x10FFFF:
		return r
	}
	return -1
}

// columnsOf returns each node's column: its distance from the entry node
// along the edges. A node the entry cannot reach goes one column past the last.
func columnsOf(s starterStructure, index map[string]int) []int {
	adjacent := make([][]int, len(s.Nodes))
	hasIncoming := make([]bool, len(s.Nodes))
	for _, edge := range s.Edges {
		from, to := index[strings.TrimSpace(edge.From)], index[strings.TrimSpace(edge.To)]
		adjacent[from] = append(adjacent[from], to)
		hasIncoming[to] = true
	}
	columns := make([]int, len(s.Nodes))
	for i := range columns {
		columns[i] = -1
	}
	entry := entryNode(s.Nodes, hasIncoming)
	columns[entry] = 0
	queue, last := []int{entry}, 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacent[current] {
			if columns[next] == -1 {
				columns[next] = columns[current] + 1
				last = max(last, columns[next])
				queue = append(queue, next)
			}
		}
	}
	for i, column := range columns {
		if column == -1 {
			columns[i] = last + 1
		}
	}
	return columns
}

// entryNode is the first client, else the first node nothing points to, else the first node.
func entryNode(nodes []starterNode, hasIncoming []bool) int {
	for i, node := range nodes {
		if strings.ToLower(strings.TrimSpace(node.Kind)) == kindClient {
			return i
		}
	}
	for i := range nodes {
		if !hasIncoming[i] {
			return i
		}
	}
	return 0
}
