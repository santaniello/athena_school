package architecture

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeDiagram_aMalformedDiagramIsInvalid(t *testing.T) {
	for name, content := range map[string]string{
		"not xml":          "hello",
		"truncated":        `<mxfile><diagram><mxGraphModel><root><mxCell id="1"`,
		"no model":         `<mxfile></mxfile>`,
		"bad compression":  `<mxfile><diagram>!!!not-base64!!!</diagram></mxfile>`,
		"not deflate data": `<mxfile><diagram>aGVsbG8gd29ybGQ=</diagram></mxfile>`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given an unreadable diagram
			// When summarizing it
			_, err := SummarizeDiagram(content)

			// Then it is invalid
			assert.ErrorIs(t, err, ErrInvalidDiagram)
		})
	}
}

func TestSummarizeDiagram_aDiagramWithNothingDrawnIsEmpty(t *testing.T) {
	for name, content := range map[string]string{
		"only the roots":      wrapFile(""),
		"unlabeled shapes":    wrapFile(`<mxCell id="a" parent="1" vertex="1" value=" "/>`),
		"an empty file":       `<mxfile><diagram></diagram></mxfile>`,
		"an empty submission": "",
	} {
		t.Run(name, func(t *testing.T) {
			// Given a diagram with nothing labeled or connected
			// When summarizing it
			_, err := SummarizeDiagram(content)

			// Then it is empty
			assert.ErrorIs(t, err, ErrEmptyDiagram)
		})
	}
}

func TestSummarizeDiagram_aDecompressionBombIsInvalid(t *testing.T) {
	// Given a small compressed page that inflates past the limit
	bomb := strings.Repeat("A", MaxDecompressedSize+1)
	file := `<mxfile><diagram>` + compressDiagram(t, bomb) + `</diagram></mxfile>`

	// When summarizing it
	_, err := SummarizeDiagram(file)

	// Then it fails as invalid instead of being inflated
	assert.ErrorIs(t, err, ErrInvalidDiagram)
}

func TestSummarizeDiagram_stopsAtTheCellCap(t *testing.T) {
	// Given a diagram with one cell more than the cap, roots included
	var cells strings.Builder
	for i := 0; i < MaxCells-1; i++ {
		fmt.Fprintf(&cells, `<mxCell id="c%d" parent="1" vertex="1" value="n%d"/>`, i, i)
	}

	// When summarizing it
	_, err := SummarizeDiagram(wrapFile(cells.String()))

	// Then it is invalid
	assert.ErrorIs(t, err, ErrInvalidDiagram)
}

func TestSummarizeDiagram_acceptsExactlyTheCellCap(t *testing.T) {
	// Given a diagram of exactly the cap, roots included
	var cells strings.Builder
	for i := 0; i < MaxCells-2; i++ {
		fmt.Fprintf(&cells, `<mxCell id="c%d" parent="1" vertex="1" value="n%d"/>`, i, i)
	}

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells.String()))

	// Then it is read
	assert.NoError(t, err)
	assert.Len(t, summary.Components, MaxCells-2)
}

func TestSummarizeDiagram_doesNotResolveEntities(t *testing.T) {
	// Given a diagram that declares an external entity and uses it as a label
	content := `<?xml version="1.0"?><!DOCTYPE mxfile [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>` +
		wrapFile(`<mxCell id="a" parent="1" vertex="1" value="&xxe;"/>`)

	// When summarizing it
	summary, err := SummarizeDiagram(content)

	// Then it is rejected and nothing from the file system is read
	assert.ErrorIs(t, err, ErrInvalidDiagram)
	assert.NotContains(t, fmt.Sprintf("%v", summary), "root:")
}

func TestSummarizeDiagram_aCompressedPageMustHoldAGraphModelDirectly(t *testing.T) {
	// Given a compressed page that itself holds another compressed page
	inner := `<mxfile><diagram>` + compressDiagram(t, wrapModel(sampleCells)) + `</diagram></mxfile>`
	file := `<mxfile><diagram>` + compressDiagram(t, inner) + `</diagram></mxfile>`

	// When summarizing it
	_, err := SummarizeDiagram(file)

	// Then the nesting is rejected instead of being inflated layer by layer
	assert.ErrorIs(t, err, ErrInvalidDiagram)
}

func TestSummarizeDiagram_acceptsAPageThatInflatesToExactlyTheLimit(t *testing.T) {
	// Given a compressed page whose inflated text is exactly MaxDecompressedSize bytes
	model := wrapModel(sampleCells)
	escapedLength := func(padding int) int {
		return len(strings.ReplaceAll(url.QueryEscape(model+"<!--"+strings.Repeat("A", padding)+"-->"), "+", "%20"))
	}
	padding := MaxDecompressedSize - escapedLength(0)
	require.Equal(t, MaxDecompressedSize, escapedLength(padding))
	file := `<mxfile><diagram>` + compressDiagram(t, model+"<!--"+strings.Repeat("A", padding)+"-->") + `</diagram></mxfile>`

	// When summarizing it
	summary, err := SummarizeDiagram(file)

	// Then it is read
	require.NoError(t, err)
	assert.Len(t, summary.Components, 3)
}

func TestSummarizeDiagram_aWrapperDoesNotLendItsLabelToTheNextCell(t *testing.T) {
	// Given an object followed by a plain cell
	cells := `<object label="Queue" id="q"><mxCell parent="1" vertex="1"/></object>
<mxCell id="p" parent="1" vertex="1" value="Plain"/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then the plain cell keeps its own label
	require.NoError(t, err)
	assert.Equal(t, []SummaryComponent{{Number: 1, Label: "Queue"}, {Number: 2, Label: "Plain"}}, summary.Components)
}

func TestSummarizeDiagram_survivesAParentCycleAmongAncestors(t *testing.T) {
	// Given a labeled cell whose ancestors loop among unlabeled cells
	cells := `<mxCell id="a" parent="b" vertex="1" value="A"/>
<mxCell id="b" parent="c" vertex="1" value=""/><mxCell id="c" parent="b" vertex="1" value=""/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then it terminates with the labeled component at the top level
	require.NoError(t, err)
	assert.Equal(t, []SummaryComponent{{Number: 1, Label: "A"}}, summary.Components)
}

func TestSummarizeDiagram_aComponentInAParentCycleWithItselfHasNoContainer(t *testing.T) {
	// Given a labeled cell whose parent chain leads back to itself
	cells := `<mxCell id="a" parent="b" vertex="1" value="A"/><mxCell id="b" parent="a" vertex="1" value=""/>`

	// When summarizing it
	summary, err := SummarizeDiagram(wrapFile(cells))

	// Then it is not its own container
	require.NoError(t, err)
	assert.Equal(t, []SummaryComponent{{Number: 1, Label: "A"}}, summary.Components)
}
