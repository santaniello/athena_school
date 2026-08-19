package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExtraction_capsEnvelopeAtConfiguredMaximumInReturnedOrder(t *testing.T) {
	// Given a valid envelope larger than the configured maximum
	raw := `{"items":[` + strings.Join([]string{
		`{"concept":"first","definition":"one"}`,
		`{"concept":"second","definition":"two"}`,
		`{"concept":"third","definition":"three"}`,
	}, ",") + `]}`

	// When parsing with a maximum of two items
	items, err := parseExtraction(raw, "Topic", 2, time.Now())

	// Then the first two are kept in their original order
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "first", items[0].Concept)
	assert.Equal(t, "second", items[1].Concept)

	// And an envelope exactly at the maximum keeps every item
	exactItems, exactErr := parseExtraction(
		`{"items":[{"concept":"first","definition":"one"},{"concept":"second","definition":"two"}]}`,
		"Topic",
		2,
		time.Now(),
	)
	require.NoError(t, exactErr)
	assert.Len(t, exactItems, 2)

	// And a minimal JSON object is still a parseable empty envelope
	emptyItems, emptyErr := parseExtraction(`{}`, "Topic", 2, time.Now())
	require.NoError(t, emptyErr)
	assert.Empty(t, emptyItems)
}

func TestParseExtraction_keepsDefinitionAtCapAndTruncatesOneOverCap(t *testing.T) {
	// Given definitions exactly at and one character over the cap
	exact := strings.Repeat("a", maxDefinitionChars)
	over := exact + "b"
	raw := fmt.Sprintf(`{"items":[{"concept":"exact","definition":%q},{"concept":"over","definition":%q}]}`, exact, over)

	// When parsing them
	items, err := parseExtraction(raw, "Topic", 8, time.Now())

	// Then the exact value remains intact and the oversized value is truncated
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, exact, items[0].Definition)
	assert.Equal(t, exact, items[1].Definition)

	// And list normalization drops blanks, caps the surviving entries at ten,
	// and truncates every entry independently
	values := []string{" "}
	for index := range 11 {
		values = append(values, fmt.Sprintf("entry-%d-%s", index, strings.Repeat("x", maxListEntryChars)))
	}
	listRaw, marshalErr := json.Marshal(map[string]any{"items": []map[string]any{{
		"concept": "lists", "definition": "bounded lists", "properties": values,
	}}})
	require.NoError(t, marshalErr)
	listItems, listErr := parseExtraction(string(listRaw), "Topic", 8, time.Now())
	require.NoError(t, listErr)
	require.Len(t, listItems, 1)
	assert.Len(t, listItems[0].Properties, maxListItems)
	for _, property := range listItems[0].Properties {
		assert.Len(t, []rune(property), maxListEntryChars)
	}
}
