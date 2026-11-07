package architecture

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wrapModel(cells string) string {
	return `<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/>` + cells + `</root></mxGraphModel>`
}

func wrapFile(cells string) string {
	return `<mxfile host="embed.diagrams.net"><diagram id="d" name="Page-1">` + wrapModel(cells) + `</diagram></mxfile>`
}

// compressDiagram mimics draw.io: encodeURIComponent, raw deflate, base64.
func compressDiagram(t *testing.T, xml string) string {
	t.Helper()
	escaped := strings.ReplaceAll(url.QueryEscape(xml), "+", "%20")
	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = writer.Write([]byte(escaped))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

const sampleCells = `
<mxCell id="vpc" parent="1" value="VPC" style="container=1;" vertex="1"><mxGeometry x="10" y="20" width="300" height="200" as="geometry"/></mxCell>
<mxCell id="api" parent="vpc" value="API" style="rounded=1;fillColor=#fff;" vertex="1"><mxGeometry x="40" y="50" as="geometry"/></mxCell>
<mxCell id="db" parent="1" value="Database" style="shape=cylinder;" vertex="1"><mxGeometry x="500" y="90" as="geometry"/></mxCell>
<mxCell id="e1" parent="1" value="reads" style="endArrow=classic;" edge="1" source="api" target="db"><mxGeometry relative="1" as="geometry"/></mxCell>`

func TestSummarizeDiagram_listsComponentsConnectionsAndGroupings(t *testing.T) {
	// Given an uncompressed diagram with a container, two components and an edge
	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(sampleCells))

	// Then components are numbered in document order, the edge keeps source, target and label,
	// and the API sits inside the VPC
	require.NoError(t, err)
	assert.Equal(t, DiagramSummary{
		Components: []SummaryComponent{
			{Number: 1, Label: "VPC"},
			{Number: 2, Label: "API", Container: 1},
			{Number: 3, Label: "Database"},
		},
		Connections: []SummaryConnection{{Source: 2, Target: 3, Label: "reads"}},
	}, summary)
}

func TestSummarizeDiagram_readsABareGraphModel(t *testing.T) {
	// Given a diagram that is just an mxGraphModel
	// When summarizing it
	summary, err := SummarizeDiagram(wrapModel(sampleCells))

	// Then it is read like a file
	require.NoError(t, err)
	assert.Len(t, summary.Components, 3)
}

func TestSummarizeDiagram_readsACompressedDiagram(t *testing.T) {
	// Given a diagram whose page content is compressed
	file := `<mxfile><diagram name="p">` + compressDiagram(t, wrapModel(sampleCells)) + `</diagram></mxfile>`

	// When summarizing it
	summary, err := SummarizeDiagram(file)

	// Then it equals the uncompressed summary
	require.NoError(t, err)
	expected, err := SummarizeDiagram(wrapFile(sampleCells))
	require.NoError(t, err)
	assert.Equal(t, expected, summary)
}

func TestSummarizeDiagram_readsObjectWrappedCells(t *testing.T) {
	// Given cells wrapped in an object that holds the label
	cells := `<object label="Queue" id="q"><mxCell parent="1" vertex="1" style=""/></object>
<UserObject label="Worker" id="w"><mxCell parent="1" vertex="1" style=""/></UserObject>
<mxCell id="e" parent="1" edge="1" source="q" target="w"/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then the labels come from the objects and the edge links them
	require.NoError(t, err)
	assert.Equal(t, []SummaryComponent{{Number: 1, Label: "Queue"}, {Number: 2, Label: "Worker"}}, summary.Components)
	assert.Equal(t, []SummaryConnection{{Source: 1, Target: 2}}, summary.Connections)
}

func TestSummarizeDiagram_turnsHTMLLabelsIntoPlainText(t *testing.T) {
	// Given an HTML label with markup, a line break and an entity
	cells := `<mxCell id="a" parent="1" vertex="1" style="html=1;" value="&lt;b&gt;Orders&lt;/b&gt;&lt;br&gt;Service &amp;amp; API"/>
<mxCell id="b" parent="1" vertex="1" style="rounded=1;" value="a &lt; b &gt; c"/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then html=1 labels lose their tags and plain labels are untouched
	require.NoError(t, err)
	assert.Equal(t, "Orders Service & API", summary.Components[0].Label)
	assert.Equal(t, "a < b > c", summary.Components[1].Label)
}

func TestSummarizeDiagram_cutsLongLabels(t *testing.T) {
	// Given a label longer than 200 characters
	cells := fmt.Sprintf(`<mxCell id="a" parent="1" vertex="1" value="%s"/>`, strings.Repeat("é", 250))

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then it is cut at 200 characters
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("é", MaxLabelLength), summary.Components[0].Label)
}

func TestSummarizeDiagram_carriesNoStylesOrCoordinates(t *testing.T) {
	// Given the sample diagram
	summary, err := SummarizeDiagram(wrapFile(sampleCells))
	require.NoError(t, err)

	// When rendering what the model will see
	rendered := fmt.Sprintf("%+v", summary)

	// Then no style or coordinate leaks
	for _, leak := range []string{"fillColor", "rounded", "cylinder", "300", "500", "geometry"} {
		assert.NotContains(t, rendered, leak)
	}
}

func TestSummarizeDiagram_keepsAnUnlabeledShapeAnEdgeUses(t *testing.T) {
	// Given an unlabeled shape linked to a labeled one, and another unlabeled shape nobody uses
	cells := `<mxCell id="a" parent="1" vertex="1" value="Client"/>
<mxCell id="b" parent="1" vertex="1" value=""/>
<mxCell id="c" parent="1" vertex="1" value=""/>
<mxCell id="e" parent="1" edge="1" source="a" target="b"/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then the used shape is listed as unlabeled and the unused one is dropped
	require.NoError(t, err)
	assert.Equal(t, []SummaryComponent{{Number: 1, Label: "Client"}, {Number: 2, Label: UnlabeledComponent}}, summary.Components)
	assert.Equal(t, []SummaryConnection{{Source: 1, Target: 2}}, summary.Connections)
}

func TestSummarizeDiagram_dropsEdgesWithADanglingEnd(t *testing.T) {
	// Given an edge that ends in nothing
	cells := `<mxCell id="a" parent="1" vertex="1" value="Client"/>
<mxCell id="e" parent="1" edge="1" source="a" value="orphan"/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then the component stays and the edge is dropped
	require.NoError(t, err)
	assert.Len(t, summary.Components, 1)
	assert.Empty(t, summary.Connections)
}

func TestSummarizeDiagram_foldsAnEdgeLabelCellIntoItsEdge(t *testing.T) {
	// Given an edge with no value whose label is a child cell
	cells := `<mxCell id="a" parent="1" vertex="1" value="A"/>
<mxCell id="b" parent="1" vertex="1" value="B"/>
<mxCell id="e" parent="1" edge="1" source="a" target="b"/>
<mxCell id="l" parent="e" vertex="1" value="publishes"/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then the label belongs to the edge and is not a component
	require.NoError(t, err)
	assert.Len(t, summary.Components, 2)
	assert.Equal(t, "publishes", summary.Connections[0].Label)
}

func TestSummarizeDiagram_readsTheEditorExamples(t *testing.T) {
	for _, name := range []string{"small", "typical", "large"} {
		t.Run(name, func(t *testing.T) {
			// Given a real editor output
			content, err := fs.ReadFile(os.DirFS("../../../specs/decisions/ADR-006-examples"), name+".drawio.xml")
			require.NoError(t, err)

			// When summarizing it
			summary, err := SummarizeDiagram(string(content))

			// Then components and connections are found
			require.NoError(t, err)
			assert.NotEmpty(t, summary.Components)
			assert.NotEmpty(t, summary.Connections)
		})
	}
}
