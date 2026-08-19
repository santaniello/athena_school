package knowledge

import (
	"fmt"
	"strings"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

const maxTranscriptChars = 24000

func buildExtractionPrompt(history []domainstudy.Message, maxItems int) (string, bool) {
	transcript, truncated := renderTranscript(history, maxTranscriptChars)
	prompt := fmt.Sprintf(`You extract durable study concepts from a transcript.
Return only valid JSON, with no markdown fences or commentary, using exactly this envelope schema:
{"items":[{"concept":"string","definition":"string","properties":["string"],"trade_offs":["string"],"related_concepts":["string"]}]}
Return at most %d items. Each definition must be self-contained and must not merely restate a question.

Transcript:
%s`, maxItems, transcript)
	return prompt, truncated
}

func renderTranscript(history []domainstudy.Message, maxChars int) (string, bool) {
	rendered := make([]string, len(history))
	for index, message := range history {
		role := "Assistant"
		if message.Role == domainstudy.RoleUser {
			role = "User"
		}
		rendered[index] = role + ": " + strings.TrimSpace(message.Content)
	}

	start := len(rendered)
	for index := len(rendered) - 1; index >= 0; index-- {
		candidate := strings.Join(rendered[index:], "\n")
		if len([]rune(candidate)) > maxChars {
			break
		}
		start = index
	}
	return strings.Join(rendered[start:], "\n"), start > 0
}
