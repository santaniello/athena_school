package architecture

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const slowPriceStructure = `{
  "nodes": [
    {"id": "app", "label": "Mobile app", "kind": "client"},
    {"id": "product", "label": "Product API", "kind": "service"},
    {"id": "price", "label": "Price API", "kind": "service"},
    {"id": "db", "label": "Prices DB", "kind": "database"}
  ],
  "edges": [
    {"from": "app", "to": "product", "label": "GET /products"},
    {"from": "product", "to": "price", "label": "300 ms"},
    {"from": "price", "to": "db"}
  ]
}`

func TestStarterDiagramRendererRoundTripsThroughTheSummary(t *testing.T) {
	// Given a structure describing four components and three calls
	// When it is rendered and then summarized like a submitted diagram
	rendered, err := StarterDiagramRenderer{}.Render([]byte(slowPriceStructure))
	require.NoError(t, err)
	summary, err := SummarizeDiagram(rendered)

	// Then every component and connection comes back, in order
	require.NoError(t, err)
	labels := []string{}
	for _, component := range summary.Components {
		labels = append(labels, component.Label)
		assert.Zero(t, component.Container)
	}
	assert.Equal(t, []string{"Mobile app", "Product API", "Price API", "Prices DB"}, labels)
	assert.Equal(t, []SummaryConnection{
		{Source: 1, Target: 2, Label: "GET /products"},
		{Source: 2, Target: 3, Label: "300 ms"},
		{Source: 3, Target: 4, Label: ""},
	}, summary.Connections)
}

func TestStarterDiagramRendererLaysNodesOutLeftToRightByDistanceFromTheEntry(t *testing.T) {
	// Given a chain starting at the client
	// When it is rendered
	rendered, err := StarterDiagramRenderer{}.Render([]byte(slowPriceStructure))
	require.NoError(t, err)

	// Then each hop sits one column to the right of the previous one
	assert.Contains(t, rendered, `<mxGeometry x="0" y="0" width="140" height="60" as="geometry"/>`)
	assert.Contains(t, rendered, `<mxGeometry x="220" y="0" width="140" height="60" as="geometry"/>`)
	assert.Contains(t, rendered, `<mxGeometry x="440" y="0" width="140" height="60" as="geometry"/>`)
	assert.Contains(t, rendered, `<mxGeometry x="660" y="0" width="140" height="60" as="geometry"/>`)
}

func TestStarterDiagramRendererStacksNodesOfTheSameColumn(t *testing.T) {
	// Given a client that calls two services
	raw := `{"nodes":[{"id":"c","label":"Client","kind":"client"},{"id":"a","label":"A","kind":"service"},{"id":"b","label":"B","kind":"service"}],
	"edges":[{"from":"c","to":"a"},{"from":"c","to":"b"}]}`

	// When it is rendered
	rendered, err := StarterDiagramRenderer{}.Render([]byte(raw))
	require.NoError(t, err)

	// Then the two services share a column and sit one row apart
	assert.Contains(t, rendered, `<mxGeometry x="220" y="0" width="140" height="60" as="geometry"/>`)
	assert.Contains(t, rendered, `<mxGeometry x="220" y="100" width="140" height="60" as="geometry"/>`)
}

func TestStarterDiagramRendererStartsFromTheClientNotFromTheFirstNode(t *testing.T) {
	// Given the client listed after the service it calls
	raw := `{"nodes":[{"id":"s","label":"Service","kind":"service"},{"id":"c","label":"Client","kind":"client"}],
	"edges":[{"from":"c","to":"s"}]}`

	// When it is rendered
	rendered, err := StarterDiagramRenderer{}.Render([]byte(raw))
	require.NoError(t, err)

	// Then the service is in the second column and the client in the first
	assert.Contains(t, rendered, `value="Service" style="`+kindStyles["service"]+`" vertex="1" parent="1"><mxGeometry x="220" y="0"`)
	assert.Contains(t, rendered, `value="Client" style="`+kindStyles["client"]+`" vertex="1" parent="1"><mxGeometry x="0" y="0"`)
}

func TestStarterDiagramRendererPlacesUnreachableNodesAfterTheLastColumn(t *testing.T) {
	// Given a node nothing connects to, and a cycle
	raw := `{"nodes":[{"id":"c","label":"Client","kind":"client"},{"id":"a","label":"A","kind":"service"},{"id":"x","label":"Island","kind":"other"}],
	"edges":[{"from":"c","to":"a"},{"from":"a","to":"c"}]}`

	// When it is rendered
	rendered, err := StarterDiagramRenderer{}.Render([]byte(raw))
	require.NoError(t, err)

	// Then it terminates and the island sits in the column after the service
	assert.Contains(t, rendered, `x="440" y="0"`)
}

func TestStarterDiagramRendererStylesEveryKindAndFallsBackToOther(t *testing.T) {
	// Given one node of each kind plus an unknown one
	kinds := []string{"client", "service", "database", "cache", "queue", "external", "other", "teleporter"}
	nodes := make([]string, len(kinds))
	for i, kind := range kinds {
		nodes[i] = fmt.Sprintf(`{"id":"n%d","label":"Node %d","kind":%q}`, i, i, kind)
	}

	// When it is rendered
	rendered, err := StarterDiagramRenderer{}.Render([]byte(`{"nodes":[` + strings.Join(nodes, ",") + `],"edges":[]}`))
	require.NoError(t, err)

	// Then each distinct kind has its own style and the unknown kind shares the other style
	for _, kind := range []string{"client", "service", "database", "cache", "queue", "external", "other"} {
		assert.Contains(t, rendered, `style="`+kindStyles[kind]+`"`, kind)
	}
	assert.Len(t, kindStyles, 7)
	assert.Equal(t, 2, strings.Count(rendered, `style="`+kindStyles["other"]+`"`))
	assert.NotContains(t, rendered, "teleporter")
}

func TestStarterDiagramRendererEscapesLabelsAndBoundsTheirLength(t *testing.T) {
	// Given a label with markup and one longer than the limit
	long := strings.Repeat("x", MaxLabelLength+50)
	raw := fmt.Sprintf(`{"nodes":[{"id":"a","label":"  A \"<b>&\" ","kind":"service"},{"id":"b","label":%q,"kind":"service"}],"edges":[{"from":"a","to":"b","label":"<i>"}]}`, long)

	// When it is rendered and read back
	rendered, err := StarterDiagramRenderer{}.Render([]byte(raw))
	require.NoError(t, err)
	summary, err := SummarizeDiagram(rendered)

	// Then the markup is plain text and the long label is cut
	require.NoError(t, err)
	assert.Equal(t, `A "<b>&"`, summary.Components[0].Label)
	assert.Equal(t, strings.Repeat("x", MaxLabelLength), summary.Components[1].Label)
	assert.Equal(t, "<i>", summary.Connections[0].Label)
}

func TestStarterDiagramRendererDropsCharactersXMLForbids(t *testing.T) {
	// Given labels with a NUL and a form feed, which XML 1.0 forbids
	raw := "{\"nodes\":[{\"id\":\"a\",\"label\":\"A\\u0000PI\\f\",\"kind\":\"service\"},{\"id\":\"b\",\"label\":\"B\",\"kind\":\"service\"}]," +
		"\"edges\":[{\"from\":\"a\",\"to\":\"b\",\"label\":\"x\\u0001y\"}]}"

	// When it is rendered and read back
	rendered, err := StarterDiagramRenderer{}.Render([]byte(raw))
	require.NoError(t, err)
	summary, err := SummarizeDiagram(rendered)

	// Then the XML is readable and the forbidden characters are gone
	require.NoError(t, err)
	assert.Equal(t, "API", summary.Components[0].Label)
	assert.Equal(t, "xy", summary.Connections[0].Label)
}

func TestStarterDiagramRendererAcceptsTheLimits(t *testing.T) {
	// Given 10 nodes and 15 edges
	nodes := make([]string, 10)
	for i := range nodes {
		nodes[i] = fmt.Sprintf(`{"id":"n%d","label":"N%d","kind":"service"}`, i, i)
	}
	edges := make([]string, 15)
	for i := range edges {
		edges[i] = fmt.Sprintf(`{"from":"n%d","to":"n%d"}`, i%9, i%9+1)
	}

	// When it is rendered
	_, err := StarterDiagramRenderer{}.Render([]byte(`{"nodes":[` + strings.Join(nodes, ",") + `],"edges":[` + strings.Join(edges, ",") + `]}`))

	// Then it is accepted
	require.NoError(t, err)
}

func TestStarterDiagramRendererRejectsABrokenStructure(t *testing.T) {
	manyNodes := make([]string, 11)
	for i := range manyNodes {
		manyNodes[i] = fmt.Sprintf(`{"id":"n%d","label":"N","kind":"service"}`, i)
	}
	manyEdges := strings.TrimSuffix(strings.Repeat(`{"from":"a","to":"b"},`, 16), ",")
	two := `{"id":"a","label":"A","kind":"service"},{"id":"b","label":"B","kind":"service"}`

	tests := map[string]string{
		"not json":          `nope`,
		"missing":           ``,
		"one node":          `{"nodes":[{"id":"a","label":"A","kind":"service"}],"edges":[]}`,
		"too many nodes":    `{"nodes":[` + strings.Join(manyNodes, ",") + `],"edges":[]}`,
		"too many edges":    `{"nodes":[` + two + `],"edges":[` + manyEdges + `]}`,
		"duplicate ids":     `{"nodes":[{"id":"a","label":"A","kind":"service"},{"id":"a","label":"B","kind":"service"}],"edges":[]}`,
		"blank id":          `{"nodes":[{"id":"a","label":"A","kind":"service"},{"id":" ","label":"B","kind":"service"}],"edges":[]}`,
		"blank label":       `{"nodes":[{"id":"a","label":" ","kind":"service"},{"id":"b","label":"B","kind":"service"}],"edges":[]}`,
		"unknown edge from": `{"nodes":[` + two + `],"edges":[{"from":"zzz","to":"b"}]}`,
		"unknown edge to":   `{"nodes":[` + two + `],"edges":[{"from":"a","to":"zzz"}]}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			// Given a structure that breaks a rule
			// When it is rendered
			_, err := StarterDiagramRenderer{}.Render([]byte(raw))

			// Then it fails as an invalid starter diagram
			require.ErrorIs(t, err, ErrInvalidStarterDiagram)
		})
	}
}

func TestStarterDiagramRendererNamesTheOffendingPosition(t *testing.T) {
	two := `{"id":"a","label":"A","kind":"service"},{"id":"b","label":"B","kind":"service"}`

	// Given a second node without an id, and a second edge to an unknown node
	_, nodeErr := StarterDiagramRenderer{}.Render([]byte(`{"nodes":[{"id":"a","label":"A"},{"id":" ","label":"B"}],"edges":[]}`))
	_, edgeErr := StarterDiagramRenderer{}.Render([]byte(`{"nodes":[` + two + `],"edges":[{"from":"a","to":"b"},{"from":"a","to":"zzz"}]}`))

	// Then each error counts from one
	assert.ErrorContains(t, nodeErr, "node 2 has no id")
	assert.ErrorContains(t, edgeErr, "edge 2 links an unknown node")
}
