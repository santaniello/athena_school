package study

import (
	"errors"
	"strings"
	"time"
)

// Sizing constants for the conversation summary (spec 2.24). They assume the
// cheap-tier model's window and are meant to be tuned in one place.
const (
	// SummaryModelWindowTokens is the context window assumed for the model
	// that produces summaries.
	SummaryModelWindowTokens = 128_000
	// SummaryInputBudgetTokens bounds the conversation text sent in one
	// summarization call, leaving room for the previous summary, the prompt
	// and the output.
	SummaryInputBudgetTokens = 32_000
	// SummaryMinTargetTokens and SummaryMaxTargetTokens clamp the summary
	// target size.
	SummaryMinTargetTokens = 1_000
	SummaryMaxTargetTokens = 4_000

	summaryTargetWindowPercent = 10
	summaryToleranceNumerator  = 5 // hard cap = target * 5/4 (+25%)
	summaryToleranceDenom      = 4
)

// ErrInvalidConversationSummary is returned by NewConversationSummary when
// an id is empty, the content is blank or the token estimate is negative.
var ErrInvalidConversationSummary = errors.New("invalid conversation summary")

// ConversationSummary is the current incremental summary of a study session.
// It is a derived artifact covering the messages up to and including
// CoversUntilMessageID; the messages themselves are never deleted. It is
// untrusted LLM output: never cited, retrieved or indexed.
type ConversationSummary struct {
	SessionID            string
	Content              string
	CoversUntilMessageID string
	TokenEstimate        int
	UpdatedAt            time.Time
}

// NewConversationSummary validates its inputs and returns the summary, or
// ErrInvalidConversationSummary if sessionID or coversUntilMessageID is
// empty, content is blank or tokenEstimate is negative.
func NewConversationSummary(sessionID, content, coversUntilMessageID string, tokenEstimate int, updatedAt time.Time) (ConversationSummary, error) {
	if sessionID == "" || coversUntilMessageID == "" || strings.TrimSpace(content) == "" || tokenEstimate < 0 {
		return ConversationSummary{}, ErrInvalidConversationSummary
	}
	return ConversationSummary{
		SessionID:            sessionID,
		Content:              content,
		CoversUntilMessageID: coversUntilMessageID,
		TokenEstimate:        tokenEstimate,
		UpdatedAt:            updatedAt,
	}, nil
}

// SummaryTargetTokens returns the size the summary should aim for: a fraction
// of the model window, clamped between SummaryMinTargetTokens and
// SummaryMaxTargetTokens.
func SummaryTargetTokens(windowTokens int) int {
	target := windowTokens * summaryTargetWindowPercent / 100
	return min(max(target, SummaryMinTargetTokens), SummaryMaxTargetTokens)
}

// SummaryHardCapTokens returns the largest summary accepted for the given
// target: the target plus a tolerance.
func SummaryHardCapTokens(targetTokens int) int {
	return targetTokens * summaryToleranceNumerator / summaryToleranceDenom
}
