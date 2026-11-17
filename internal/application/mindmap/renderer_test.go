package mindmap

import (
	"encoding/xml"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type drawnCell struct {
	ID       string `xml:"id,attr"`
	Value    string `xml:"value,attr"`
	Style    string `xml:"style,attr"`
	Vertex   string `xml:"vertex,attr"`
	Edge     string `xml:"edge,attr"`
	Source   string `xml:"source,attr"`
	Target   string `xml:"target,attr"`
	Geometry struct {
		X      int `xml:"x,attr"`
		Y      int `xml:"y,attr"`
		Width  int `xml:"width,attr"`
		Height int `xml:"height,attr"`
	} `xml:"mxGeometry"`
}

type drawnModel struct {
	Cells []drawnCell `xml:"root>mxCell"`
}

func parseDrawn(t *testing.T, doc string) (vertices map[string]drawnCell, edges []drawnCell) {
	t.Helper()
	var model drawnModel
	require.NoError(t, xml.Unmarshal([]byte(doc), &model), "the output must be well-formed XML")
	vertices = map[string]drawnCell{}
	for _, cell := range model.Cells {
		switch {
		case cell.Vertex == "1":
			vertices[cell.ID] = cell
		case cell.Edge == "1":
			edges = append(edges, cell)
		}
	}
	return vertices, edges
}

func TestRenderDrawsOneVertexPerNodeAndOneEdgePerLink(t *testing.T) {
	// Given a tree of 4 nodes
	raw := []byte(`{"label":"Caching","children":[{"label":"Eviction","children":[{"label":"LRU"}]},{"label":"Writes"}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then there is a vertex per node and an edge per parent-child link
	require.NoError(t, err)
	vertices, edges := parseDrawn(t, doc)
	assert.Len(t, vertices, 4)
	assert.Len(t, edges, 3)
	assert.Equal(t, "Caching", vertices["n0"].Value)
	assert.Equal(t, "Eviction", vertices["n1"].Value)
	assert.Equal(t, "LRU", vertices["n2"].Value)
	assert.Equal(t, "Writes", vertices["n3"].Value)
}

func TestRenderLinksEachChildToItsParent(t *testing.T) {
	// Given a tree where LRU hangs under Eviction
	raw := []byte(`{"label":"Caching","children":[{"label":"Eviction","children":[{"label":"LRU"}]},{"label":"Writes"}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then every edge goes from the parent to the child
	require.NoError(t, err)
	_, edges := parseDrawn(t, doc)
	links := map[string]string{}
	for _, edge := range edges {
		links[edge.Target] = edge.Source
	}
	assert.Equal(t, map[string]string{"n1": "n0", "n2": "n1", "n3": "n0"}, links)
}

func TestRenderPlacesDepthsInColumnsAndLeavesInRows(t *testing.T) {
	// Given a root with two leaves
	raw := []byte(`{"label":"Root","children":[{"label":"A"},{"label":"B"}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then the root is in column 0, the leaves share column 1 in successive rows,
	// and the root is vertically centred between them
	require.NoError(t, err)
	vertices, _ := parseDrawn(t, doc)
	assert.Equal(t, 0, vertices["n0"].Geometry.X)
	assert.Equal(t, columnSpread, vertices["n1"].Geometry.X)
	assert.Equal(t, columnSpread, vertices["n2"].Geometry.X)
	assert.Equal(t, 0, vertices["n1"].Geometry.Y)
	assert.Equal(t, rowSpread, vertices["n2"].Geometry.Y)
	assert.Equal(t, rowSpread/2, vertices["n0"].Geometry.Y)
	assert.Equal(t, nodeWidth, vertices["n0"].Geometry.Width)
	assert.Equal(t, nodeHeight, vertices["n0"].Geometry.Height)
}

func TestRenderKeepsLeavesOfDifferentBranchesInSeparateRows(t *testing.T) {
	// Given two branches with two leaves each
	raw := []byte(`{"label":"R","children":[{"label":"A","children":[{"label":"a1"},{"label":"a2"}]},{"label":"B","children":[{"label":"b1"},{"label":"b2"}]}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then the four leaves take four rows, in order
	require.NoError(t, err)
	vertices, _ := parseDrawn(t, doc)
	assert.Equal(t, 0*rowSpread, vertices["n2"].Geometry.Y)
	assert.Equal(t, 1*rowSpread, vertices["n3"].Geometry.Y)
	assert.Equal(t, 2*rowSpread, vertices["n5"].Geometry.Y)
	assert.Equal(t, 3*rowSpread, vertices["n6"].Geometry.Y)
	assert.Equal(t, rowSpread/2, vertices["n1"].Geometry.Y, "A is centred over a1 and a2")
}

func TestRenderStylesByDepthAndColoursEachBranch(t *testing.T) {
	// Given a root with two branches and a leaf under the first
	raw := []byte(`{"label":"R","children":[{"label":"A","children":[{"label":"a1"}]},{"label":"B"}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then the root has its own style, branches differ in colour, and a leaf
	// shares its branch's colour
	require.NoError(t, err)
	vertices, _ := parseDrawn(t, doc)
	assert.Equal(t, rootStyle, vertices["n0"].Style)
	assert.NotEqual(t, vertices["n1"].Style, vertices["n3"].Style, "branches have different colours")
	assert.Contains(t, vertices["n2"].Style, branchPalette[0].stroke, "a leaf keeps its branch colour")
	assert.Contains(t, vertices["n1"].Style, "strokeWidth=2;", "first-level branches have a bolder border")
	assert.Contains(t, vertices["n2"].Style, "strokeWidth=1;", "deeper nodes have a plain border")
}

func TestRenderReusesThePaletteWhenThereAreMoreBranchesThanColours(t *testing.T) {
	// Given more top-level branches than the palette has colours
	children := make([]string, len(branchPalette)+1)
	for i := range children {
		children[i] = `{"label":"b"}`
	}
	raw := []byte(`{"label":"R","children":[` + strings.Join(children, ",") + `]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then the colours wrap around
	require.NoError(t, err)
	vertices, _ := parseDrawn(t, doc)
	assert.Equal(t, vertices["n1"].Style, vertices["n"+strconv.Itoa(len(branchPalette)+1)].Style)
}

func TestRenderEscapesAndCleansLabels(t *testing.T) {
	// Given labels with markup, quotes, ampersands and a control character
	raw := []byte(`{"label":"<R & \"D\">","children":[{"label":"a\u0000b"}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then the output is well-formed and the labels are intact text
	require.NoError(t, err)
	vertices, _ := parseDrawn(t, doc)
	assert.Equal(t, `<R & "D">`, vertices["n0"].Value)
	assert.Equal(t, "ab", vertices["n1"].Value)
}

func TestRenderTurnsLineBreaksIntoSpaces(t *testing.T) {
	// Given a label with a line break and a tab
	raw := []byte(`{"label":"Write-through\nvs\twrite-back","children":[{"label":"a"}]}`)

	// When it is rendered
	doc, err := Renderer{}.Render(raw)

	// Then the label reads on one line, as the editor would show it anyway
	require.NoError(t, err)
	assert.NotContains(t, doc, "\n")
	vertices, _ := parseDrawn(t, doc)
	assert.Equal(t, "Write-through vs write-back", vertices["n0"].Value)
}

func TestRenderWrapsInvalidTrees(t *testing.T) {
	// Given a reply that is not a valid tree
	// When it is rendered
	_, err := Renderer{}.Render([]byte(`{"label":"alone"}`))

	// Then the error is ErrInvalidTree
	assert.ErrorIs(t, err, ErrInvalidTree)
}
