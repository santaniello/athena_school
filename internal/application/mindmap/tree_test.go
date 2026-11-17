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

func TestCleanLabelDropsForbiddenCharactersTrimsAndBounds(t *testing.T) {
	// Given labels with control characters, spaces and an overlong text
	long := strings.Repeat("é", MaxLabelLength+20)

	// When they are cleaned
	// Then forbidden runes go, spaces are trimmed and the length is bounded by runes
	assert.Equal(t, "ab", CleanLabel(" a\x00b\x01 "))
	assert.Equal(t, "a\tb", CleanLabel("a\tb"))
	assert.Equal(t, strings.Repeat("é", MaxLabelLength), CleanLabel(long))
}

func TestCleanLabelKeepsExactlyTheRunesXMLAllows(t *testing.T) {
	tests := []struct {
		name string
		in   rune
		kept bool
	}{
		{"tab", '\t', true},
		{"line feed", '\n', true},
		{"carriage return", '\r', true},
		{"last control character", 0x1F, false},
		{"first printable", 0x20, true},
		{"end of the first range", 0xD7FF, true},
		{"start of the second range", 0xE000, true},
		{"end of the second range", 0xFFFD, true},
		{"first past the second range", 0xFFFE, false},
		{"start of the third range", 0x10000, true},
		{"end of the third range", 0x10FFFF, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a label with the rune between two letters
			label := "a" + string(tt.in) + "b"

			// When it is cleaned
			cleaned := CleanLabel(label)

			// Then the rune is kept only if XML 1.0 allows it
			if tt.kept {
				assert.Equal(t, label, cleaned)
			} else {
				assert.Equal(t, "ab", cleaned)
			}
		})
	}
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
