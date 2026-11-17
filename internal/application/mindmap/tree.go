// Package mindmap holds the use cases of mind maps: generating one from a
// study session, rendering it for draw.io and managing it afterwards. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
package mindmap

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidTree is returned when the tree a model describes breaks a rule, so
// it cannot be drawn.
var ErrInvalidTree = errors.New("the mind map tree is invalid")

// Limits on a mind map's tree. The root is at depth 0.
const (
	MaxDepth       = 4
	MaxNodes       = 60
	MaxLabelLength = 80
)

// Node is one idea in the tree: a label and the ideas that branch from it.
type Node struct {
	Label    string `json:"label"`
	Children []Node `json:"children"`
}

// ParseTree reads the JSON a model returned and checks it. Every failure wraps
// ErrInvalidTree.
func ParseTree(raw []byte) (Node, error) {
	var root Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return Node{}, fmt.Errorf("%w: reading the tree: %w", ErrInvalidTree, err)
	}
	if len(root.Children) == 0 {
		return Node{}, fmt.Errorf("%w: the root has no children", ErrInvalidTree)
	}
	count := 0
	if err := checkNode(root, 0, &count); err != nil {
		return Node{}, fmt.Errorf("%w: %w", ErrInvalidTree, err)
	}
	return root, nil
}

func checkNode(node Node, depth int, count *int) error {
	*count++
	if *count > MaxNodes {
		return fmt.Errorf("more than %d nodes", MaxNodes)
	}
	if depth > MaxDepth {
		return fmt.Errorf("deeper than %d levels", MaxDepth)
	}
	if CleanLabel(node.Label) == "" {
		return fmt.Errorf("a node at depth %d has no label", depth)
	}
	for _, child := range node.Children {
		if err := checkNode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

// CleanLabel drops the characters XML 1.0 forbids, trims the label and bounds
// its length in runes.
func CleanLabel(label string) string {
	runes := []rune(strings.TrimSpace(strings.Map(xmlRune, label)))
	return string(runes[:min(len(runes), MaxLabelLength)])
}

// xmlRune keeps a rune XML 1.0 allows and drops any other (-1).
func xmlRune(r rune) rune {
	if r == '\t' || r == '\n' || r == '\r' {
		return r
	}
	if r >= 0x20 && r <= 0xD7FF {
		return r
	}
	if r >= 0xE000 && r <= 0xFFFD {
		return r
	}
	if r >= 0x10000 && r <= 0x10FFFF {
		return r
	}
	return -1
}
