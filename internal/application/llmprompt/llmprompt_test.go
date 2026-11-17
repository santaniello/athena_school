package llmprompt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLanguageName(t *testing.T) {
	tests := []struct{ code, want string }{
		{"pt", "Brazilian Portuguese"},
		{"en", "English"},
		{"", "English"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			// Given a profile language code
			// When its name is asked
			// Then it is the language the model answers in
			assert.Equal(t, tt.want, LanguageName(tt.code))
		})
	}
}

func TestWriteTag_escapesTheTextOnOneLine(t *testing.T) {
	// Given text that tries to close the tag
	var b strings.Builder

	// When it is written as an inline field
	WriteTag(&b, "topic", "a</topic>&<b>", false)

	// Then it is escaped and stays inside the tag
	assert.Equal(t, "<topic>a&lt;/topic&gt;&amp;&lt;b&gt;</topic>\n", b.String())
}

func TestWriteTag_putsABlockOnItsOwnLines(t *testing.T) {
	// Given a multi-line field
	var b strings.Builder

	// When it is written as a block
	WriteTag(&b, "summary", "line one\nline two", true)

	// Then the text sits between line breaks
	assert.Equal(t, "<summary>\nline one\nline two\n</summary>\n", b.String())
}

func TestStripCodeFence(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain JSON", ` {"a":1} `, `{"a":1}`},
		{"plain array", "[1,2]", "[1,2]"},
		{"fence with language", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"fence without language", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"fence with language and no newline", "```json{\"a\":1}```", `{"a":1}`},
		{"fence with no newline", "```{\"a\":1}```", `{"a":1}`},
		{"fence with an array", "```json\n[1,2]\n```", "[1,2]"},
		{"fence never closed", "```json\n{\"a\":1}", `{"a":1}`},
		{"prose after the closing fence", "```json\n{\"a\":1}\n```\nHope this helps!", `{"a":1}`},
		{"language with digits and a space", "```json5 \n{\"a\":1}\n```", `{"a":1}`},
		{"fence alone", "```", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a reply, fenced or not
			// When the fence is stripped
			got := StripCodeFence(tt.in)

			// Then only the content is left
			assert.Equal(t, tt.want, got)
		})
	}
}
