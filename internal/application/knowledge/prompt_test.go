package knowledge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

func TestBuildExtractionPrompt_marksMessagesAndRequiresBoundedVerbatimEvidence(t *testing.T) {
	// Given a Study Session transcript with identifiable user and assistant Messages
	history := []domainstudy.Message{
		{ID: "message-user", Role: domainstudy.RoleUser, Content: "What is a channel?"},
		{ID: "message-assistant", Role: domainstudy.RoleAssistant, Content: "A channel coordinates communication."},
	}

	// When building the extraction prompt
	prompt, includedMessages, truncated, err := buildExtractionPrompt(history, 8)

	// Then every Message is marked and the response schema requires bounded literal EvidenceRefs
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, history, includedMessages)
	assert.Contains(t, prompt, "[message:message-user] User:\nWhat is a channel?")
	assert.Contains(t, prompt, "[message:message-assistant] Assistant:\nA channel coordinates communication.")
	assert.Contains(t, prompt, `"evidence":[{"message_id":"string","quote":"string"}]`)
	assert.Contains(t, prompt, "at least one and at most 5 evidence references")
	assert.Contains(t, prompt, "at most 1000 Unicode characters")
	assert.Contains(t, strings.ToLower(prompt), "copied verbatim")
}
