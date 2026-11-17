// Package drawio holds the helpers shared by the renderers that write draw.io
// mxGraphModel XML.
package drawio

import "strings"

// CleanText makes text safe for an mxCell value: it drops the characters XML
// 1.0 forbids, turns tabs and line breaks into spaces (XML attribute
// normalisation would do it anyway, silently) and trims the result. It does
// not escape: callers escape after they bound the length.
func CleanText(text string) string {
	return strings.TrimSpace(strings.Map(xmlRune, text))
}

// xmlRune keeps a rune XML 1.0 allows, turns whitespace an attribute value
// cannot keep into a space and drops any other rune (-1).
func xmlRune(r rune) rune {
	switch {
	case r == '\t', r == '\n', r == '\r':
		return ' '
	case r >= 0x20 && r <= 0xD7FF,
		r >= 0xE000 && r <= 0xFFFD,
		r >= 0x10000 && r <= 0x10FFFF:
		return r
	}
	return -1
}
