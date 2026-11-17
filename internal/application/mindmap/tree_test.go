package mindmap

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTreeReadsLabelsAndChildren(t *testing.T) {
	// Given a model reply with a root and nested children
	raw := []byte(`{"label":"Caching","children":[{"label":"Eviction","children":[{"label":"LRU"}]},{"label":"Writes"}]}`)

	// When it is parsed
	root, err := ParseTree(raw)

	// Then the structure is kept
	require.NoError(t, err)
	assert.Equal(t, "Caching", root.Label)
	require.Len(t, root.Children, 2)
	assert.Equal(t, "Eviction", root.Children[0].Label)
	assert.Equal(t, "LRU", root.Children[0].Children[0].Label)
	assert.Equal(t, "Writes", root.Children[1].Label)
}

func TestParseTreeRejectsBrokenTrees(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"not json", `not json`},
		{"empty root label", `{"label":"  ","children":[{"label":"a"}]}`},
		{"root without children", `{"label":"Caching"}`},
		{"empty child label", `{"label":"Caching","children":[{"label":""}]}`},
		{"label with only forbidden characters", `{"label":"Caching","children":[{"label":"\u0000\u0001"}]}`},
		{"too deep", deepTree(MaxDepth + 1)},
		{"too many nodes", wideTree(MaxNodes)},
		{"root label too long", `{"label":"` + strings.Repeat("é", MaxLabelLength+1) + `","children":[{"label":"a"}]}`},
		{"child label too long", `{"label":"r","children":[{"label":"` + strings.Repeat("x", MaxLabelLength+1) + `"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a reply that breaks a rule
			// When it is parsed
			_, err := ParseTree([]byte(tt.raw))

			// Then it is an invalid mind map tree
			assert.ErrorIs(t, err, ErrInvalidTree)
		})
	}
}

func TestParseTreeAcceptsTheLimits(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"deepest allowed", deepTree(MaxDepth)},
		{"most nodes allowed", wideTree(MaxNodes - 1)},
		{"smallest tree", `{"label":"a","children":[{"label":"b"}]}`},
		{"longest label allowed", `{"label":"` + strings.Repeat("é", MaxLabelLength) + `","children":[{"label":"b"}]}`},
		{"long label once cleaned", `{"label":"  ` + strings.Repeat("x", MaxLabelLength) + `  \u0000","children":[{"label":"b"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a reply right at a limit
			// When it is parsed
			_, err := ParseTree([]byte(tt.raw))

			// Then it is accepted
			assert.NoError(t, err)
		})
	}
}

func TestParseTreeReturnsCleanedLabels(t *testing.T) {
	// Given a reply whose labels have spaces, control characters and line breaks
	raw := []byte(`{"label":"  Caching\u0001 ","children":[{"label":"Write-through\nvs write-back","children":[{"label":"\tLRU"}]}]}`)

	// When it is parsed
	root, err := ParseTree(raw)

	// Then every label in the returned tree is already clean
	require.NoError(t, err)
	assert.Equal(t, "Caching", root.Label)
	assert.Equal(t, "Write-through vs write-back", root.Children[0].Label)
	assert.Equal(t, "LRU", root.Children[0].Children[0].Label)
}

func TestCleanLabelDropsForbiddenCharactersAndTrims(t *testing.T) {
	// Given a label with control characters, a line break and spaces
	// When it is cleaned
	// Then forbidden runes go, the line break is a space and the label is trimmed
	assert.Equal(t, "a b", CleanLabel(" a\x00\nb\x01 "))
}

// deepTree builds a chain whose deepest node is at the given depth (the root is 0).
func deepTree(depth int) string {
	var b strings.Builder
	for i := 0; i <= depth; i++ {
		fmt.Fprintf(&b, `{"label":"n%d"`, i)
		if i < depth {
			b.WriteString(`,"children":[`)
		}
	}
	for i := depth; i >= 0; i-- {
		if i < depth {
			b.WriteString(`]`)
		}
		b.WriteString(`}`)
	}
	return b.String()
}

// wideTree builds a root with the given number of children, so it has children+1 nodes.
func wideTree(children int) string {
	items := make([]string, children)
	for i := range items {
		items[i] = fmt.Sprintf(`{"label":"c%d"}`, i)
	}
	return `{"label":"root","children":[` + strings.Join(items, ",") + `]}`
}
