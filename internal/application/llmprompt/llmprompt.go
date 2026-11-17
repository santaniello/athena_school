// Package llmprompt holds the small helpers every feature that prompts a model
// shares: naming the answer language, wrapping untrusted text in tags and
// reading a JSON answer out of a Markdown code fence.
package llmprompt

import (
	"fmt"
	"strings"
)

// LanguageName returns the name of the language a model should answer in for a
// profile's language code: Brazilian Portuguese for "pt", English otherwise.
func LanguageName(code string) string {
	if code == "pt" {
		return "Brazilian Portuguese"
	}
	return "English"
}

// WriteTag writes <name>text</name> followed by a line break, with the text
// escaped so it can neither close the tag nor open another. A block field puts
// the text on its own lines.
func WriteTag(b *strings.Builder, name, text string, block bool) {
	separator := ""
	if block {
		separator = "\n"
	}
	fmt.Fprintf(b, "<%s>%s%s%s</%s>\n", name, separator, EscapeTag(text), separator, name)
}

// EscapeTag makes text safe to place between tags.
func EscapeTag(text string) string {
	return tagEscaper.Replace(text)
}

var tagEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

const fence = "```"

// StripCodeFence returns what a Markdown code fence some models wrap around
// their answer holds. An answer that does not start with a fence comes back
// trimmed. Otherwise the opening fence and its info string ("json"), with or
// without a line break after it, are dropped, and so are the closing fence and
// anything written after it. A fence that is never closed keeps the rest of the
// answer.
func StripCodeFence(content string) string {
	text := strings.TrimSpace(content)
	if !strings.HasPrefix(text, fence) {
		return text
	}
	text = strings.TrimLeft(text[len(fence):], infoStringChars)
	if body, _, closed := strings.Cut(text, fence); closed {
		text = body
	}
	return strings.TrimSpace(text)
}

// infoStringChars are the characters an info string such as "json" or
// "json5" is made of. None of them can start a JSON object or array.
const infoStringChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+-"
