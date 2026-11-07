package architecture

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// diagramError is a diagram failure that also knows the short, user-facing
// reason an attempt is stored with. The exercise engine reads it through its
// FailureReason method without importing this package.
type diagramError struct {
	message string
	reason  string
}

func (e *diagramError) Error() string { return e.message }

// FailureReason returns the short reason shown to the user.
func (e *diagramError) FailureReason() string { return e.reason }

// Errors returned by SummarizeDiagram and the evaluator.
var (
	// ErrInvalidDiagram is returned when the submitted diagram cannot be read:
	// wrong content type, malformed XML, corrupt or oversized compression, or
	// too many cells.
	ErrInvalidDiagram error = &diagramError{
		message: "the diagram could not be read", reason: "The diagram could not be read.",
	}
	// ErrEmptyDiagram is returned when the diagram holds no labeled component
	// and no connection, so there is nothing to evaluate.
	ErrEmptyDiagram error = &diagramError{
		message: "the diagram is empty", reason: "The diagram is empty.",
	}
)

// Limits on untrusted diagram input.
const (
	// MaxDecompressedSize is the most a compressed diagram may inflate to, in bytes.
	MaxDecompressedSize = 8 << 20
	// MaxCells is the most cells a diagram may hold.
	MaxCells = 2000
	// MaxLabelLength is the longest label kept, in characters.
	MaxLabelLength = 200
)

// UnlabeledComponent stands for a shape without a label that a connection uses.
const UnlabeledComponent = "(unlabeled)"

// SummaryComponent is one shape of the diagram. Container is the Number of the
// component it sits inside, or 0 when it sits at the top level.
type SummaryComponent struct {
	Number    int
	Label     string
	Container int
}

// SummaryConnection links two components by Number.
type SummaryConnection struct {
	Source int
	Target int
	Label  string
}

// DiagramSummary is what the evaluation model sees of a diagram: components,
// connections and groupings, with no styles, coordinates or editor ids.
type DiagramSummary struct {
	Components  []SummaryComponent
	Connections []SummaryConnection
}

type diagramCell struct {
	id, parent, label, style string
	vertex, edge             bool
	source, target           string
}

// SummarizeDiagram turns draw.io XML (an mxfile, compressed or not, or a bare
// mxGraphModel) into a DiagramSummary. The content is untrusted: nothing is
// resolved from it, the inflated size and the cell count are capped, and
// labels are plain text of bounded length. Pure and deterministic. See
// specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md.
func SummarizeDiagram(content string) (DiagramSummary, error) {
	if strings.TrimSpace(content) == "" {
		return DiagramSummary{}, ErrEmptyDiagram
	}
	cells, err := readCells(xml.NewDecoder(strings.NewReader(content)))
	if err != nil {
		return DiagramSummary{}, err
	}
	summary := buildSummary(cells)
	if len(summary.Components) == 0 && len(summary.Connections) == 0 {
		return DiagramSummary{}, ErrEmptyDiagram
	}
	return summary, nil
}

func invalidDiagram(cause error) error {
	return fmt.Errorf("%w: %w", ErrInvalidDiagram, cause)
}

// readCells finds the first diagram page and returns its cells. A page either
// holds an mxGraphModel or the compressed text of one.
func readCells(dec *xml.Decoder) ([]diagramCell, error) {
	var compressed strings.Builder
	inDiagram := false
	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil, invalidDiagram(errors.New("no diagram page found"))
		}
		if err != nil {
			return nil, invalidDiagram(err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "mxGraphModel":
				return readModel(dec)
			case "diagram":
				inDiagram = true
			}
		case xml.CharData:
			if inDiagram {
				compressed.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "diagram" {
				return readCompressed(strings.TrimSpace(compressed.String()))
			}
		}
	}
}

func readCompressed(text string) ([]diagramCell, error) {
	if text == "" {
		return nil, ErrEmptyDiagram
	}
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, invalidDiagram(err)
	}
	inflated, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), MaxDecompressedSize+1))
	if err != nil {
		return nil, invalidDiagram(err)
	}
	if len(inflated) > MaxDecompressedSize {
		return nil, invalidDiagram(errors.New("the diagram is too large once decompressed"))
	}
	model, err := url.PathUnescape(string(inflated))
	if err != nil {
		return nil, invalidDiagram(err)
	}
	return readCells(xml.NewDecoder(strings.NewReader(model)))
}

// readModel reads the cells of an mxGraphModel whose start tag was consumed.
// A cell wrapped in object or UserObject takes its id and label from the wrapper.
func readModel(dec *xml.Decoder) ([]diagramCell, error) {
	var cells []diagramCell
	var wrapper *xml.StartElement
	depth := 1
	for depth > 0 {
		token, err := dec.Token()
		if err != nil {
			return nil, invalidDiagram(err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			switch t.Name.Local {
			case "object", "UserObject":
				wrapper = &t
			case "mxCell":
				if len(cells) >= MaxCells {
					return nil, invalidDiagram(fmt.Errorf("more than %d cells", MaxCells))
				}
				cells = append(cells, newCell(t, wrapper))
			}
		case xml.EndElement:
			depth--
			if t.Name.Local == "object" || t.Name.Local == "UserObject" {
				wrapper = nil
			}
		}
	}
	return cells, nil
}

func newCell(element xml.StartElement, wrapper *xml.StartElement) diagramCell {
	cell := diagramCell{}
	for _, attr := range element.Attr {
		switch attr.Name.Local {
		case "id":
			cell.id = attr.Value
		case "parent":
			cell.parent = attr.Value
		case "value":
			cell.label = attr.Value
		case "style":
			cell.style = attr.Value
		case "vertex":
			cell.vertex = attr.Value == "1"
		case "edge":
			cell.edge = attr.Value == "1"
		case "source":
			cell.source = attr.Value
		case "target":
			cell.target = attr.Value
		}
	}
	if wrapper != nil {
		for _, attr := range wrapper.Attr {
			switch attr.Name.Local {
			case "id":
				cell.id = attr.Value
			case "label":
				cell.label = attr.Value
			}
		}
	}
	return cell
}

var (
	lineBreaks = regexp.MustCompile(`(?i)<br\s*/?>|</(div|p|li)>`)
	htmlTags   = regexp.MustCompile(`<[^>]*>`)
	whitespace = regexp.MustCompile(`\s+`)
)

// plainLabel returns the label as bounded plain text. Only a shape styled
// html=1 holds markup; any other label is taken as typed.
func plainLabel(cell diagramCell) string {
	text := cell.label
	if strings.Contains(";"+cell.style, ";html=1") {
		text = html.UnescapeString(htmlTags.ReplaceAllString(lineBreaks.ReplaceAllString(text, " "), ""))
	}
	text = strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
	if utf8.RuneCountInString(text) > MaxLabelLength {
		text = string([]rune(text)[:MaxLabelLength])
	}
	return text
}

func buildSummary(cells []diagramCell) DiagramSummary {
	byID := make(map[string]diagramCell, len(cells))
	edgeLabels := map[string][]string{}
	for _, cell := range cells {
		byID[cell.id] = cell
	}
	used := map[string]bool{}
	for _, cell := range cells {
		if cell.edge {
			used[cell.source], used[cell.target] = true, true
		}
	}
	numbers := map[string]int{}
	var summary DiagramSummary
	for _, cell := range cells {
		if !cell.vertex {
			continue
		}
		label := plainLabel(cell)
		if parent, ok := byID[cell.parent]; ok && parent.edge {
			if label != "" {
				edgeLabels[parent.id] = append(edgeLabels[parent.id], label)
			}
			continue
		}
		if label == "" && !used[cell.id] {
			continue
		}
		if label == "" {
			label = UnlabeledComponent
		}
		numbers[cell.id] = len(summary.Components) + 1
		summary.Components = append(summary.Components, SummaryComponent{Number: numbers[cell.id], Label: label})
	}
	assignContainers(cells, byID, numbers, &summary)
	for _, cell := range cells {
		source, target := numbers[cell.source], numbers[cell.target]
		if !cell.edge || source == 0 || target == 0 {
			continue
		}
		label := plainLabel(cell)
		if label == "" {
			label = strings.Join(edgeLabels[cell.id], " ")
		}
		summary.Connections = append(summary.Connections, SummaryConnection{Source: source, Target: target, Label: label})
	}
	return summary
}

// assignContainers sets each component's Container to the nearest ancestor
// that is itself a component. The walk is bounded so a parent cycle cannot hang it.
func assignContainers(cells []diagramCell, byID map[string]diagramCell, numbers map[string]int, summary *DiagramSummary) {
	for _, cell := range cells {
		number := numbers[cell.id]
		if !cell.vertex || number == 0 {
			continue
		}
		parent := cell.parent
		for step := 0; step < len(cells); step++ {
			if container := numbers[parent]; container != 0 {
				summary.Components[number-1].Container = container
				break
			}
			next, ok := byID[parent]
			if !ok {
				break
			}
			parent = next.parent
		}
	}
}
