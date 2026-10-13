package webfetch

import (
	"io"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// skippedElements never contribute readable text: code, chrome and form
// controls rather than the page's content.
var skippedElements = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Nav: true,
	atom.Footer: true, atom.Aside: true, atom.Form: true, atom.Iframe: true,
	atom.Svg: true, atom.Template: true, atom.Head: true, atom.Button: true,
	atom.Select: true, atom.Textarea: true, atom.Canvas: true,
}

// blockElements start and end a paragraph-sized block of text.
var blockElements = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Section: true, atom.Article: true,
	atom.Main: true, atom.Header: true, atom.Blockquote: true, atom.Table: true,
	atom.Figure: true, atom.Figcaption: true, atom.Dl: true, atom.Dt: true,
	atom.Dd: true, atom.Address: true, atom.Details: true, atom.Summary: true,
	atom.Hr: true,
}

// chromeRoles are ARIA roles of page furniture rather than content.
var chromeRoles = map[string]bool{
	"navigation": true, "banner": true, "contentinfo": true, "complementary": true, "search": true,
}

// isSkipped reports whether n is an element that never contributes readable
// text: a noise tag, a region ARIA marks as page chrome, or one hidden from
// everyone.
func isSkipped(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if skippedElements[n.DataAtom] {
		return true
	}
	for _, attr := range n.Attr {
		switch attr.Key {
		case "role":
			if chromeRoles[strings.ToLower(attr.Val)] {
				return true
			}
		case "hidden":
			return true
		case "aria-hidden":
			if strings.EqualFold(attr.Val, "true") {
				return true
			}
		}
	}
	return false
}

var headingLevels = map[atom.Atom]int{
	atom.H1: 1, atom.H2: 2, atom.H3: 3, atom.H4: 4, atom.H5: 5, atom.H6: 6,
}

// htmlToMarkdown converts an HTML document to the page's title and its
// readable text as Markdown. It deliberately favors a simple, predictable
// result over fidelity: the text later goes through the note-formatting
// pass and the user's review.
func htmlToMarkdown(r io.Reader) (title, markdown string, err error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", "", err
	}

	w := &markdownWriter{}
	w.walk(contentRoot(doc))
	return pageTitle(doc), strings.TrimSpace(w.out.String()), nil
}

// pageTitle returns the <title> text, falling back to the first <h1>.
func pageTitle(doc *html.Node) string {
	if n := findFirst(doc, func(n *html.Node) bool { return n.DataAtom == atom.Title }); n != nil {
		if title := normalizedText(n); title != "" {
			return title
		}
	}
	if n := findFirst(doc, func(n *html.Node) bool { return n.DataAtom == atom.H1 }); n != nil {
		return normalizedText(n)
	}
	return ""
}

// contentRoot picks the page's main content region: the article or main
// element with the most text, so unrelated sidebars and teasers are left
// out; the body when the page marks no such region.
func contentRoot(doc *html.Node) *html.Node {
	var best *html.Node
	bestLen := 0
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.DataAtom == atom.Article || n.DataAtom == atom.Main) {
			if length := len(normalizedText(n)); length > bestLen {
				best, bestLen = n, length
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	if best != nil {
		return best
	}
	if body := findFirst(doc, func(n *html.Node) bool { return n.DataAtom == atom.Body }); body != nil {
		return body
	}
	return doc
}

func findFirst(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirst(c, match); found != nil {
			return found
		}
	}
	return nil
}

// normalizedText is n's text with every whitespace run collapsed to one
// space, skipping noise elements.
func normalizedText(n *html.Node) string {
	return strings.Join(strings.Fields(rawText(n)), " ")
}

// rawText is n's text exactly as written, skipping noise elements.
func rawText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if isSkipped(n) {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return b.String()
}

type listState struct {
	ordered bool
	next    int
}

// markdownWriter accumulates Markdown. sep is the separator owed before the
// next piece of text, so blank lines never pile up and never lead the output.
type markdownWriter struct {
	out strings.Builder
	sep string
	// afterMarker is true right after a list marker, so a block element
	// directly inside the item stays on the marker's line.
	afterMarker bool
	lists       []*listState
}

func (w *markdownWriter) emit(s string) {
	if s == "" {
		return
	}
	if w.out.Len() > 0 {
		w.out.WriteString(w.sep)
	}
	w.sep = ""
	w.afterMarker = false
	w.out.WriteString(s)
}

func (w *markdownWriter) block() {
	if !w.afterMarker {
		w.sep = "\n\n"
	}
}

func (w *markdownWriter) line() {
	if !w.afterMarker && w.sep != "\n\n" {
		w.sep = "\n"
	}
}

func (w *markdownWriter) space() {
	if w.sep == "" {
		w.sep = " "
	}
}

func (w *markdownWriter) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		w.text(n.Data)
	case html.ElementNode:
		if isSkipped(n) {
			return
		}
		w.element(n)
	default:
		w.children(n)
	}
}

func (w *markdownWriter) children(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.walk(c)
	}
}

func (w *markdownWriter) text(data string) {
	fields := strings.Fields(data)
	if len(fields) == 0 {
		if data != "" {
			w.space()
		}
		return
	}
	if strings.TrimLeft(data, " \t\r\n\f") != data {
		w.space()
	}
	w.emit(strings.Join(fields, " "))
	if strings.TrimRight(data, " \t\r\n\f") != data {
		w.space()
	}
}

func (w *markdownWriter) element(n *html.Node) {
	if level, ok := headingLevels[n.DataAtom]; ok {
		w.heading(n, level)
		return
	}
	switch n.DataAtom {
	case atom.Pre:
		w.preformatted(n)
	case atom.Code:
		if text := normalizedText(n); text != "" {
			w.emit("`" + text + "`")
		}
	case atom.Br:
		w.line()
	case atom.Ul, atom.Ol:
		w.list(n)
	case atom.Li:
		w.listItem(n)
	case atom.Tr:
		w.line()
		w.children(n)
		w.line()
	case atom.Td, atom.Th:
		w.space()
		w.children(n)
	default:
		if blockElements[n.DataAtom] {
			w.block()
			w.children(n)
			w.block()
			return
		}
		w.children(n)
	}
}

func (w *markdownWriter) heading(n *html.Node, level int) {
	text := normalizedText(n)
	if text == "" {
		return
	}
	w.block()
	w.emit(strings.Repeat("#", level) + " " + text)
	w.block()
}

func (w *markdownWriter) preformatted(n *html.Node) {
	text := strings.Trim(rawText(n), "\r\n")
	if strings.TrimSpace(text) == "" {
		return
	}
	w.block()
	w.emit("```\n" + text + "\n```")
	w.block()
}

func (w *markdownWriter) list(n *html.Node) {
	if len(w.lists) > 0 {
		w.line()
	} else {
		w.block()
	}
	w.lists = append(w.lists, &listState{ordered: n.DataAtom == atom.Ol, next: 1})
	w.children(n)
	w.lists = w.lists[:len(w.lists)-1]
	if len(w.lists) > 0 {
		w.line()
	} else {
		w.block()
	}
}

func (w *markdownWriter) listItem(n *html.Node) {
	marker := "-"
	indent := ""
	if len(w.lists) > 0 {
		current := w.lists[len(w.lists)-1]
		if current.ordered {
			marker = strconv.Itoa(current.next) + "."
			current.next++
		}
		indent = strings.Repeat("  ", len(w.lists)-1)
	}
	w.line()
	w.emit(indent + marker)
	w.sep = " "
	w.afterMarker = true
	w.children(n)
	w.line()
}
