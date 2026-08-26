package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

func parseTestMessages() []domainstudy.Message {
	return []domainstudy.Message{{ID: "message-1", Content: "support"}}
}

const parseTestEvidence = `,"evidence":[{"message_id":"message-1","quote":"support"}]`

func TestParseExtraction_capsEnvelopeAtConfiguredMaximumInReturnedOrder(t *testing.T) {
	// Given a valid envelope larger than the configured maximum
	raw := `{"items":[` + strings.Join([]string{
		`{"concept":"first","definition":"one"` + parseTestEvidence + `}`,
		`{"concept":"second","definition":"two"` + parseTestEvidence + `}`,
		`{"concept":"third","definition":"three"` + parseTestEvidence + `}`,
	}, ",") + `]}`

	// When parsing with a maximum of two items
	items, err := parseExtraction(raw, "Topic", 2, time.Now(), parseTestMessages())

	// Then the first two are kept in their original order
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "first", items[0].Concept)
	assert.Equal(t, "second", items[1].Concept)

	// And an envelope exactly at the maximum keeps every item
	exactItems, exactErr := parseExtraction(
		`{"items":[{"concept":"first","definition":"one"`+parseTestEvidence+`},{"concept":"second","definition":"two"`+parseTestEvidence+`}]}`,
		"Topic",
		2,
		time.Now(),
		parseTestMessages(),
	)
	require.NoError(t, exactErr)
	assert.Len(t, exactItems, 2)

	// And a minimal JSON object is still a parseable empty envelope
	emptyItems, emptyErr := parseExtraction(`{}`, "Topic", 2, time.Now(), parseTestMessages())
	require.NoError(t, emptyErr)
	assert.Empty(t, emptyItems)
}

func TestParseExtraction_keepsDefinitionAtCapAndTruncatesOneOverCap(t *testing.T) {
	// Given definitions exactly at and one character over the cap
	exact := strings.Repeat("a", maxDefinitionChars)
	over := exact + "b"
	raw := fmt.Sprintf(`{"items":[{"concept":"exact","definition":%q%s},{"concept":"over","definition":%q%s}]}`, exact, parseTestEvidence, over, parseTestEvidence)

	// When parsing them
	items, err := parseExtraction(raw, "Topic", 8, time.Now(), parseTestMessages())

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
		"evidence": []map[string]string{{"message_id": "message-1", "quote": "support"}},
	}}})
	require.NoError(t, marshalErr)
	listItems, listErr := parseExtraction(string(listRaw), "Topic", 8, time.Now(), parseTestMessages())
	require.NoError(t, listErr)
	require.Len(t, listItems, 1)
	assert.Len(t, listItems[0].Properties, maxListItems)
	for _, property := range listItems[0].Properties {
		assert.Len(t, []rune(property), maxListEntryChars)
	}
}

func TestParseExtraction_keepsTrimmedVerbatimEvidenceFromIncludedMessage(t *testing.T) {
	// Given an extracted candidate quoting a Message included in the capped transcript
	messages := []domainstudy.Message{{ID: "message-1", Content: "A channel coordinates communication."}}
	raw := `{"items":[{"concept":"Channels","definition":"Typed conduits.","evidence":[{"message_id":"message-1","quote":"  A channel coordinates communication.  "}]}]}`

	// When parsing the extraction
	candidates, err := parseExtraction(raw, "Go", 8, time.Now(), messages)

	// Then the candidate keeps the literal quote with only its edges trimmed
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, []domainknowledge.EvidenceRef{{
		MessageID: "message-1",
		Quote:     "A channel coordinates communication.",
	}}, candidates[0].EvidenceRefs)
}

func TestParseExtraction_skipsCandidateWithoutValidEvidenceAndKeepsValidSibling(t *testing.T) {
	// Given one candidate whose references are all invalid and a sibling with
	// one valid reference plus duplicated, unknown, blank, over-limit, and
	// non-verbatim references
	overLimit := strings.Repeat("é", maxEvidenceQuoteChars+1)
	messages := []domainstudy.Message{
		{ID: "message-1", Content: "valid quote " + overLimit},
		{ID: "message-at-limit", Content: strings.Repeat("ç", maxEvidenceQuoteChars)},
	}
	raw, marshalErr := json.Marshal(map[string]any{"items": []map[string]any{
		{
			"concept": "invalid", "definition": "no evidence",
			"evidence": []map[string]string{
				{"message_id": "message-outside-transcript", "quote": "valid quote"},
				{"message_id": "message-1", "quote": " "},
				{"message_id": "message-1", "quote": "not verbatim"},
				{"message_id": "message-1", "quote": overLimit},
			},
		},
		{
			"concept": "valid", "definition": "has evidence",
			"evidence": []map[string]string{
				{"message_id": "message-1", "quote": "valid quote"},
				{"message_id": "message-1", "quote": " valid quote "},
				{"message_id": "message-outside-transcript", "quote": "valid quote"},
				{"message_id": "message-1", "quote": " "},
				{"message_id": "message-1", "quote": "not verbatim"},
				{"message_id": "message-1", "quote": overLimit},
			},
		},
		{
			"concept": "at limit", "definition": "literal quote at limit",
			"evidence": []map[string]string{{
				"message_id": "message-at-limit", "quote": strings.Repeat("ç", maxEvidenceQuoteChars),
			}},
		},
	}})
	require.NoError(t, marshalErr)

	// When parsing the extraction
	candidates, err := parseExtraction(string(raw), "Go", 8, time.Now(), messages)

	// Then the invalid candidate is skipped while valid siblings keep only
	// their accepted, de-duplicated references
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	assert.Equal(t, "valid", candidates[0].Concept)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "valid quote"}}, candidates[0].EvidenceRefs)
	assert.Equal(t, "at limit", candidates[1].Concept)
	require.Len(t, candidates[1].EvidenceRefs, 1)
	assert.Len(t, []rune(candidates[1].EvidenceRefs[0].Quote), maxEvidenceQuoteChars)
}

func TestParseExtraction_capsEvidenceRefsAtMaxEvidencePerItemEvenWithMoreIndependentlyValidRefs(t *testing.T) {
	// Given a candidate citing six independently valid references — each its
	// own message, none blank, over-limit, duplicated, or non-verbatim
	messages := make([]domainstudy.Message, 0, maxEvidencePerItem+1)
	evidence := make([]map[string]string, 0, maxEvidencePerItem+1)
	for i := 0; i < maxEvidencePerItem+1; i++ {
		messageID := fmt.Sprintf("message-%d", i)
		quote := fmt.Sprintf("quote number %d", i)
		messages = append(messages, domainstudy.Message{ID: messageID, Content: quote})
		evidence = append(evidence, map[string]string{"message_id": messageID, "quote": quote})
	}
	raw, marshalErr := json.Marshal(map[string]any{"items": []map[string]any{
		{"concept": "capped", "definition": "six references", "evidence": evidence},
	}})
	require.NoError(t, marshalErr)

	// When parsing the extraction
	candidates, err := parseExtraction(string(raw), "Go", 8, time.Now(), messages)

	// Then only the first maxEvidencePerItem references are kept, in order —
	// a mutated cap comparison (e.g. stopping after the very first valid ref)
	// would fail this exact count and content
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Len(t, candidates[0].EvidenceRefs, maxEvidencePerItem)
	for i := 0; i < maxEvidencePerItem; i++ {
		assert.Equal(t, domainknowledge.EvidenceRef{
			MessageID: fmt.Sprintf("message-%d", i),
			Quote:     fmt.Sprintf("quote number %d", i),
		}, candidates[0].EvidenceRefs[i])
	}
}
