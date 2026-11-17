package drawio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanTextDropsForbiddenCharactersAndTrims(t *testing.T) {
	// Given text with control characters and surrounding spaces
	// When it is cleaned
	// Then forbidden runes go and the spaces around it are trimmed
	assert.Equal(t, "ab", CleanText(" a\x00b\x01 "))
	assert.Equal(t, "", CleanText("\x00\x01"))
}

func TestCleanTextTurnsLineBreaksAndTabsIntoSpaces(t *testing.T) {
	// Given text with a tab, a line feed and a carriage return
	// When it is cleaned
	// Then each becomes a space, as an attribute value would render it
	assert.Equal(t, "a b c d", CleanText("a\tb\nc\rd"))
	assert.Equal(t, "a", CleanText("\na\t"))
}

func TestCleanTextKeepsExactlyTheRunesXMLAllows(t *testing.T) {
	tests := []struct {
		name string
		in   rune
		kept bool
	}{
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
			// Given text with the rune between two letters
			text := "a" + string(tt.in) + "b"

			// When it is cleaned
			cleaned := CleanText(text)

			// Then the rune is kept only if XML 1.0 allows it
			if tt.kept {
				assert.Equal(t, text, cleaned)
			} else {
				assert.Equal(t, "ab", cleaned)
			}
		})
	}
}
