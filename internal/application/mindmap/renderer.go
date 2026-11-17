package mindmap

import (
	"fmt"
	"html"
	"strings"
)

// Layout of the rendered diagram, in draw.io units: one column per depth and
// one row per leaf.
const (
	nodeWidth    = 160
	nodeHeight   = 50
	columnSpread = 240
	rowSpread    = 70
)

const (
	rootStyle = "rounded=1;whiteSpace=wrap;fontStyle=1;fontSize=16;fillColor=#1f2937;strokeColor=#111827;fontColor=#ffffff;"
	edgeStyle = "edgeStyle=entityRelationEdgeStyle;rounded=1;endArrow=none;strokeWidth=2;strokeColor=%s;"
)

// branchColour is the colour of one top-level branch and of everything under it.
type branchColour struct{ fill, stroke string }

// branchPalette is a closed set: branch i uses entry i modulo its length.
var branchPalette = []branchColour{
	{"#dae8fc", "#6c8ebf"},
	{"#d5e8d4", "#82b366"},
	{"#ffe6cc", "#d79b00"},
	{"#e1d5e7", "#9673a6"},
	{"#f8cecc", "#b85450"},
}

// Renderer turns the tree a model describes into a valid draw.io mxGraphModel
// with a left-to-right layout. The model never writes XML: coordinates, styles
// and ids are ours.
type Renderer struct{}

// Render reads the JSON tree and returns the mxGraphModel XML. Every failure
// wraps ErrInvalidTree.
func (Renderer) Render(raw []byte) (string, error) {
	root, err := ParseTree(raw)
	if err != nil {
		return "", err
	}
	return draw(layout(root)), nil
}

// placed is a node with its position, its branch and the index of its parent
// (-1 for the root).
type placed struct {
	label  string
	depth  int
	branch int
	parent int
	y      int
}

// layout places the nodes in pre-order, so a node's index is its id: a leaf
// takes the next free row and a parent is centred over its first and last child.
func layout(root Node) []placed {
	var cells []placed
	row := 0
	var walk func(node Node, depth, branch, parent int) int
	walk = func(node Node, depth, branch, parent int) int {
		index := len(cells)
		cells = append(cells, placed{label: node.Label, depth: depth, branch: branch, parent: parent})
		if len(node.Children) == 0 {
			cells[index].y = row * rowSpread
			row++
			return index
		}
		last := 0
		for i, child := range node.Children {
			childBranch := branch
			if depth == 0 {
				childBranch = i
			}
			last = walk(child, depth+1, childBranch, index)
		}
		first := index + 1 // pre-order: the first child comes right after its parent
		cells[index].y = (cells[first].y + cells[last].y) / 2
		return index
	}
	walk(root, 0, 0, -1)
	return cells
}

func draw(cells []placed) string {
	var b strings.Builder
	b.WriteString(`<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/>`)
	for i, cell := range cells {
		fmt.Fprintf(&b,
			`<mxCell id="n%d" value="%s" style="%s" vertex="1" parent="1"><mxGeometry x="%d" y="%d" width="%d" height="%d" as="geometry"/></mxCell>`,
			i, html.EscapeString(CleanLabel(cell.label)), styleOf(cell), cell.depth*columnSpread, cell.y, nodeWidth, nodeHeight)
	}
	for i, cell := range cells {
		if cell.parent < 0 {
			continue
		}
		fmt.Fprintf(&b,
			`<mxCell id="e%d" style="%s" edge="1" parent="1" source="n%d" target="n%d"><mxGeometry relative="1" as="geometry"/></mxCell>`,
			i, fmt.Sprintf(edgeStyle, colourOf(cell).stroke), cell.parent, i)
	}
	b.WriteString(`</root></mxGraphModel>`)
	return b.String()
}

func colourOf(cell placed) branchColour {
	return branchPalette[cell.branch%len(branchPalette)]
}

// styleOf is the root's own style, or the branch colour with a bolder border
// for the first level and a plain one below.
func styleOf(cell placed) string {
	if cell.depth == 0 {
		return rootStyle
	}
	colour := colourOf(cell)
	width := 1
	if cell.depth == 1 {
		width = 2
	}
	return fmt.Sprintf("rounded=1;whiteSpace=wrap;fillColor=%s;strokeColor=%s;strokeWidth=%d;", colour.fill, colour.stroke, width)
}
