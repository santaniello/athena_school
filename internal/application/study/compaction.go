package study

import (
	"context"
	"errors"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// knowledgeContextAllowanceTokens reserves room for the retrieved-knowledge
// system message, which is only built after the user message is persisted and
// therefore cannot be measured when compaction is decided. It covers the
// 8,000-character retrieved-data budget plus the framing text.
const knowledgeContextAllowanceTokens = 3000

// ConversationSummarizer folds study messages into the session's incremental
// summary. Defined here (consumer side); implemented by *Summarizer.
type ConversationSummarizer interface {
	Summarize(ctx context.Context, sessionID string, previous *domainstudy.ConversationSummary, messages []domainstudy.Message) (domainstudy.ConversationSummary, error)
}

// WithSummarizer sets the summarizer behind context compaction. Compaction is
// active only when WithSummaries is set too.
func WithSummarizer(summarizer ConversationSummarizer) Option {
	return func(s *Service) { s.summarizer = summarizer }
}

// WithSummaries sets the repository holding each session's summary.
func WithSummaries(summaries domainstudy.ConversationSummaryRepository) Option {
	return func(s *Service) { s.summaries = summaries }
}

// CompactionHooks are the notifications of a compaction attempt. Every field
// is optional. They never abort the turn.
type CompactionHooks struct {
	// OnStarted fires right before the summarizer is called.
	OnStarted func()
	// OnCompacted fires after the summary and the reset context usage are
	// persisted, with the new usage.
	OnCompacted ContextCallback
	// OnFailed fires when compaction was needed but did not happen, with the
	// cause (see Summarizer's errors, domainstudy.ErrBudgetImpossible, or a
	// persistence error).
	OnFailed func(err error)
}

// SendOption customizes one SendMessage call.
type SendOption func(*sendConfig)

type sendConfig struct {
	compaction CompactionHooks
}

// WithCompactionHooks sets the compaction notifications of a SendMessage call.
func WithCompactionHooks(hooks CompactionHooks) SendOption {
	return func(c *sendConfig) { c.compaction = hooks }
}

// compactionEnabled reports whether the service was wired with everything
// compaction needs; otherwise sessions behave exactly as in spec 2.6.
func (s *Service) compactionEnabled() bool {
	return s.summarizer != nil && s.summaries != nil
}

// compactionOutcome is what compact learned about the session.
type compactionOutcome struct {
	// summary is the session's current summary after the call, nil when none.
	summary *domainstudy.ConversationSummary
	// session is the session as it stands after the call.
	session domainstudy.Session
	// compacted is true when this call folded messages into the summary.
	compacted bool
	// stateReset is true when nothing could be folded but re-measuring the
	// prompt lowered a warning/blocked state (typically one set by native
	// usage that counted the reply as well).
	stateReset bool
	// cause is why compaction was needed but did not happen; nil otherwise.
	cause error
	// overBudget is true when, with cause set, the prompt as it stands does
	// not fit the window minus the output reserve.
	overBudget bool
}

// compact folds the oldest messages of sessionID into its summary when the
// next prompt (including the pending user message content, which is not yet
// persisted) would cross the warning threshold or the session is already
// warning/blocked — or unconditionally when force is set. The caller must hold
// the session's in-flight reservation. A returned error is a hard failure
// (unknown session, repository failure, or a pending message that cannot fit
// the window); a compaction that merely fails is reported through
// outcome.cause and the hooks, with nothing persisted.
func (s *Service) compact(ctx context.Context, sessionID, pendingContent string, force bool, hooks CompactionHooks) (compactionOutcome, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return compactionOutcome{}, err
	}
	outcome := compactionOutcome{session: session}

	summary, err := s.summaries.Get(ctx, sessionID)
	if err == nil {
		outcome.summary = &summary
	} else if !errors.Is(err, domainstudy.ErrConversationSummaryNotFound) {
		return compactionOutcome{}, fmt.Errorf("study: loading summary: %w", err)
	}

	window := session.Context.ContextLength
	if window <= 0 {
		return outcome, nil
	}

	cursor := ""
	if outcome.summary != nil {
		cursor = outcome.summary.CoversUntilMessageID
	}
	history, err := s.messages.ListAfter(ctx, sessionID, cursor)
	if err != nil {
		return compactionOutcome{}, fmt.Errorf("study: loading history: %w", err)
	}
	profile, err := s.profiles.Load()
	if err != nil {
		return compactionOutcome{}, fmt.Errorf("study: loading profile: %w", err)
	}

	input := domainstudy.BuildInput{
		Messages:      history,
		ContextLength: window,
		FixedTokens:   domainstudy.EstimateTokens(buildSystemPrompt(profile, session.Topic, session.Goal)) + knowledgeContextAllowanceTokens,
	}
	if pendingContent != "" {
		input.PendingTokens = domainstudy.EstimateTokens(pendingContent)
	}
	if outcome.summary != nil {
		input.SummaryTokens = outcome.summary.TokenEstimate
	}

	plan, err := domainstudy.BuildContext(input)
	if errors.Is(err, domainstudy.ErrBudgetImpossible) {
		return s.failCompaction(outcome, err, true, hooks), nil
	}
	if err != nil {
		return compactionOutcome{}, err
	}

	needed := force || session.Context.State != domainstudy.ContextStateNormal ||
		domainstudy.NeedsCompaction(plan.UnfoldedTokens, window)
	if !needed {
		return outcome, nil
	}
	if plan.FoldCount == 0 {
		return s.remeasure(ctx, sessionID, session, plan.UnfoldedTokens-input.PendingTokens, window, outcome, hooks), nil
	}

	overBudget := plan.UnfoldedTokens > domainstudy.PromptBudget(window)
	if hooks.OnStarted != nil {
		hooks.OnStarted()
	}
	folded := history[:plan.FoldCount]
	newSummary, err := s.summarizer.Summarize(ctx, sessionID, outcome.summary, folded)
	if err != nil {
		return s.failCompaction(outcome, err, overBudget, hooks), nil
	}

	// The pending message is counted by SendMessage's own provisional
	// increment, so it is excluded from the reset measurement. The summary may
	// be larger than BuildContext projected, so the prompt is checked again.
	used := input.FixedTokens + newSummary.TokenEstimate + tokensOf(history[plan.FoldCount:])
	if used+input.PendingTokens > domainstudy.PromptBudget(window) {
		return s.failCompaction(outcome, domainstudy.ErrBudgetImpossible, true, hooks), nil
	}
	newUsage := domainstudy.ContextUsageAfterCompaction(session.Context, used, window)
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.summaries.Save(ctx, newSummary); err != nil {
			return err
		}
		return s.sessions.UpdateContext(ctx, sessionID, newUsage)
	})
	if err != nil {
		return s.failCompaction(outcome, fmt.Errorf("study: persisting compaction: %w", err), overBudget, hooks), nil
	}

	outcome.summary = &newSummary
	outcome.session.Context = newUsage
	outcome.compacted = true
	if hooks.OnCompacted != nil {
		hooks.OnCompacted(ContextEvent{
			State: newUsage.State, UsedTokens: newUsage.UsedTokens,
			ContextLength: newUsage.ContextLength, Estimated: newUsage.Estimated,
		})
	}
	return outcome, nil
}

// remeasure handles a non-normal session with nothing to fold: it persists the
// usage re-estimated from the actual prompt, which is the only way out of a
// state set by native usage (input + output) that overstates the next prompt.
// A normal session is left untouched.
func (s *Service) remeasure(ctx context.Context, sessionID string, session domainstudy.Session, used, window int, outcome compactionOutcome, hooks CompactionHooks) compactionOutcome {
	if session.Context.State == domainstudy.ContextStateNormal {
		return outcome
	}
	usage := domainstudy.ContextUsageAfterCompaction(session.Context, used, window)
	if err := s.sessions.UpdateContext(ctx, sessionID, usage); err != nil {
		return s.failCompaction(outcome, fmt.Errorf("study: persisting compaction: %w", err), false, hooks)
	}
	outcome.session.Context = usage
	outcome.stateReset = usage.State != session.Context.State
	return outcome
}

func (s *Service) failCompaction(outcome compactionOutcome, cause error, overBudget bool, hooks CompactionHooks) compactionOutcome {
	outcome.cause = cause
	outcome.overBudget = overBudget
	if hooks.OnFailed != nil {
		hooks.OnFailed(cause)
	}
	return outcome
}

// CompactSession is the explicit retry: it folds the oldest messages of
// sessionID into its summary regardless of how full the window is, under the
// same in-flight reservation as a turn. It returns the cause when compaction
// fails and ErrNothingToCompact when there is nothing to fold.
func (s *Service) CompactSession(ctx context.Context, sessionID string, hooks CompactionHooks) error {
	if !s.compactionEnabled() {
		return ErrNothingToCompact
	}
	if err := s.inFlight.begin(sessionID); err != nil {
		return err
	}
	defer s.inFlight.end(sessionID)

	outcome, err := s.compact(ctx, sessionID, "", true, hooks)
	if err != nil {
		return err
	}
	if outcome.cause != nil {
		return outcome.cause
	}
	if !outcome.compacted && !outcome.stateReset {
		return ErrNothingToCompact
	}
	return nil
}

func tokensOf(messages []domainstudy.Message) int {
	total := 0
	for _, message := range messages {
		total += domainstudy.EstimateTokens(message.Content)
	}
	return total
}
