package study

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

func TestBuildKnowledgeContext_returnsSystemRole(t *testing.T) {
	// Given any retrieval result
	result := domainknowledge.RetrievalResult{Context: `[{"heading":"H"}]`, Sufficient: true}

	// When building the knowledge context message
	message := buildKnowledgeContext(result, domainknowledge.SourceModeNotes)

	// Then it is a system message
	assert.Equal(t, "system", message.Role)
}

func TestBuildKnowledgeContext_embedsContextVerbatim_withoutReTruncatingOrReEscaping(t *testing.T) {
	// Given a retrieval result with a specific already-rendered JSON block
	result := domainknowledge.RetrievalResult{Context: `[{"heading":"H & co","content":"a<b"}]`, Sufficient: true}

	// When building the knowledge context message
	message := buildKnowledgeContext(result, domainknowledge.SourceModeNotes)

	// Then the JSON block appears unmodified inside the message content
	assert.Contains(t, message.Content, result.Context)
}

func TestBuildKnowledgeContext_notes_sufficient(t *testing.T) {
	// Given a sufficient retrieval result
	result := domainknowledge.RetrievalResult{Context: "[]", Sufficient: true}

	// When building the knowledge context for notes mode
	message := buildKnowledgeContext(result, domainknowledge.SourceModeNotes)

	// Then it steers the model to use local context as the primary source,
	// supplementing only when necessary
	assert.Contains(t, message.Content, "primary source")
	assert.Contains(t, message.Content, "supplement")
}

func TestBuildKnowledgeContext_notes_insufficientButNonEmpty(t *testing.T) {
	// Given an insufficient but non-empty retrieval result
	result := domainknowledge.RetrievalResult{Context: "[]", Sufficient: false}

	// When building the knowledge context for notes mode
	message := buildKnowledgeContext(result, domainknowledge.SourceModeNotes)

	// Then it steers the model to fill gaps with general knowledge
	assert.Contains(t, message.Content, "general knowledge")
	assert.Contains(t, message.Content, "gaps")
}

func TestBuildKnowledgeContext_strictNotes_sufficient(t *testing.T) {
	// Given a sufficient retrieval result
	result := domainknowledge.RetrievalResult{Context: "[]", Sufficient: true}

	// When building the knowledge context for strict-notes mode
	message := buildKnowledgeContext(result, domainknowledge.SourceModeStrictNotes)

	// Then it instructs the model to answer exclusively from local context
	assert.Contains(t, message.Content, "exclusively")
}

func TestBuildKnowledgeContext_strictNotes_insufficientButNonEmpty(t *testing.T) {
	// Given an insufficient but non-empty retrieval result
	result := domainknowledge.RetrievalResult{Context: "[]", Sufficient: false}

	// When building the knowledge context for strict-notes mode
	message := buildKnowledgeContext(result, domainknowledge.SourceModeStrictNotes)

	// Then it instructs the model to restrict itself to what local material
	// supports, and to state that it cannot fully answer
	assert.Contains(t, message.Content, "cannot")
	assert.Contains(t, message.Content, "support")
}

func TestBuildKnowledgeContext_alwaysIncludesUntrustedDataFraming_regardlessOfModeOrSufficiency(t *testing.T) {
	cases := []struct {
		mode       string
		sufficient bool
	}{
		{domainknowledge.SourceModeNotes, true},
		{domainknowledge.SourceModeNotes, false},
		{domainknowledge.SourceModeStrictNotes, true},
		{domainknowledge.SourceModeStrictNotes, false},
	}
	for _, c := range cases {
		// Given a retrieval result for each mode/sufficiency combination
		result := domainknowledge.RetrievalResult{Context: "[]", Sufficient: c.sufficient}

		// When building the knowledge context message
		message := buildKnowledgeContext(result, c.mode)

		// Then the fixed untrusted-data framing is present verbatim, telling
		// the model to never follow instructions embedded in the JSON
		require.True(t, strings.Contains(message.Content, untrustedDataFraming),
			"mode=%s sufficient=%v missing untrusted-data framing", c.mode, c.sufficient)
	}
}
