package study

import (
	"context"
	"fmt"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Option customizes optional collaborators of a Service.
type Option func(*Service)

// WithUsageReader sets the reader behind SessionUsage.
func WithUsageReader(reader domainllm.UsageReader) Option {
	return func(s *Service) { s.usage = reader }
}

// SessionUsageSnapshot pairs the two independent metrics of a session: the
// accumulated consumption of its recorded LLM calls and the persisted
// occupancy of the model's context window. study:context-* events only fire
// on state transitions, so the snapshot is what lets the UI refresh the
// context meter after every turn.
type SessionUsageSnapshot struct {
	Totals  domainllm.UsageTotals
	Context domainstudy.ContextUsage
}

// SessionUsage returns the tokens and cost accumulated by every LLM call
// recorded for sessionID (chat, HyDE and embeddings), together with the
// session's persisted context usage. Totals only cover calls that
// completed: an interrupted call leaves no usage row. See
// specs/phases/phase-02-knowledge-engine/22-usage-visibility.md.
func (s *Service) SessionUsage(ctx context.Context, sessionID string) (SessionUsageSnapshot, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return SessionUsageSnapshot{}, fmt.Errorf("study: finding session: %w", err)
	}
	totals, err := s.usage.SessionTotals(ctx, sessionID)
	if err != nil {
		return SessionUsageSnapshot{}, fmt.Errorf("study: reading session usage: %w", err)
	}
	return SessionUsageSnapshot{Totals: totals, Context: session.Context}, nil
}
