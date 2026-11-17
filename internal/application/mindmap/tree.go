// Package mindmap holds the use cases of mind maps: generating one from a
// study session, rendering it for draw.io and managing it afterwards. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
package mindmap

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/santaniello/athena/internal/application/drawio"
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

// ParseTree reads the JSON a model returned, checks it and returns it with
// every label cleaned (see CleanLabel). Every failure wraps ErrInvalidTree.
func ParseTree(raw []byte) (Node, error) {
	var root Node
	if err := json.Unmarshal(raw, &root); err != nil {
		return Node{}, fmt.Errorf("%w: reading the tree: %w", ErrInvalidTree, err)
	}
	if len(root.Children) == 0 {
		return Node{}, fmt.Errorf("%w: the root has no children", ErrInvalidTree)
	}
	count := 0
	cleaned, err := cleanNode(root, 0, &count)
	if err != nil {
		return Node{}, fmt.Errorf("%w: %w", ErrInvalidTree, err)
	}
	return cleaned, nil
}

// cleanNode checks the node and its subtree against the limits and returns a
// copy whose labels are cleaned.
func cleanNode(node Node, depth int, count *int) (Node, error) {
	*count++
	if *count > MaxNodes {
		return Node{}, fmt.Errorf("more than %d nodes", MaxNodes)
	}
	if depth > MaxDepth {
		return Node{}, fmt.Errorf("deeper than %d levels", MaxDepth)
	}
	label := CleanLabel(node.Label)
	if label == "" {
		return Node{}, fmt.Errorf("a node at depth %d has no label", depth)
	}
	if utf8.RuneCountInString(label) > MaxLabelLength {
		return Node{}, fmt.Errorf("a label at depth %d is longer than %d characters", depth, MaxLabelLength)
	}
	cleaned := Node{Label: label}
	for _, child := range node.Children {
		cleanedChild, err := cleanNode(child, depth+1, count)
		if err != nil {
			return Node{}, err
		}
		cleaned.Children = append(cleaned.Children, cleanedChild)
	}
	return cleaned, nil
}

// CleanLabel makes a label safe for draw.io: it drops the characters XML 1.0
// forbids, turns tabs and line breaks into spaces and trims it. It does not
// bound the length: ParseTree rejects a label over MaxLabelLength.
func CleanLabel(label string) string {
	return drawio.CleanText(label)
}
