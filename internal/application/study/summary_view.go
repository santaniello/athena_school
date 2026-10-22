package study

import (
	"context"
	"errors"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// GetSummary returns the current summary of sessionID. found is false, with a
// nil error, when the session has no summary yet or the service runs without
// compaction. The content is untrusted LLM output and must be shown as data.
func (s *Service) GetSummary(ctx context.Context, sessionID string) (summary domainstudy.ConversationSummary, found bool, err error) {
	if s.summaries == nil {
		return domainstudy.ConversationSummary{}, false, nil
	}
	summary, err = s.summaries.Get(ctx, sessionID)
	if errors.Is(err, domainstudy.ErrConversationSummaryNotFound) {
		return domainstudy.ConversationSummary{}, false, nil
	}
	if err != nil {
		return domainstudy.ConversationSummary{}, false, fmt.Errorf("study: reading summary: %w", err)
	}
	return summary, true, nil
}
