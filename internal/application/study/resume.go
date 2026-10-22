package study

import (
	"context"
	"fmt"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// MessageWithSources pairs a persisted domainstudy.Message with the local
// knowledge Sources that backed it, if any — attached here in the
// application layer rather than on domainstudy.Message itself, since
// domain/study has no dependency on domain/knowledge (see
// specs/phases/phase-02-knowledge-engine/09-persistent-provenance.md). A
// user message, or an assistant message that used no local knowledge
// (a strict-notes miss, the opening turn), carries a nil Sources.
type MessageWithSources struct {
	domainstudy.Message
	Sources []domainknowledge.Source
}

// SessionHistory is the page Resume returns: the latest slice of the history
// plus the cursor of the session's summary, so the transcript can mark where
// older messages were folded into it. SummaryCoversUntilMessageID is empty
// when the session has no summary.
type SessionHistory struct {
	MessageHistoryPage
	SummaryCoversUntilMessageID string
}

// Resume returns sessionID and the most recent page of its message history
// (DefaultMessagePageSize messages; older ones are read with
// ListMessagesBefore), so the user can keep chatting in it. Never makes an LLM call and never waits on catalog I/O:
// if the session's ContextLength is unresolved (0), a cache hit is
// recomputed and persisted before this returns so the DTO already reflects
// it; a cache miss with a known model starts or joins a background refresh
// (see resolveContextLengthInBackground) without blocking the return; a
// session with no resolved model at all just surfaces the transient
// unavailable notice, since there is no trustworthy ID to refresh against.
// Resume is a read: a failure to persist a freshly resolved context length
// degrades to the transient unavailable notice, and a failure to read the
// summary to no marker, instead of denying access to the session/history
// already loaded — the next real measurement persists it again regardless.
func (s *Service) Resume(
	ctx context.Context, sessionID string,
	onContext ContextCallback, onContextUnavailable ContextUnavailableCallback,
) (domainstudy.Session, SessionHistory, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, SessionHistory{}, fmt.Errorf("study: finding session: %w", err)
	}

	page, err := s.readMessagePage(ctx, sessionID, nil, DefaultMessagePageSize)
	if err != nil {
		return domainstudy.Session{}, SessionHistory{}, err
	}

	// The summary only places the transcript marker: failing to read it must
	// not deny access to a session whose history already loaded, so it
	// degrades to "no marker". GetSummary reports the error to whoever asks
	// for the summary explicitly.
	summary, _, err := s.GetSummary(ctx, sessionID)
	if err != nil {
		summary = domainstudy.ConversationSummary{}
	}

	if session.Context.ContextLength == 0 {
		switch session.Context.Model {
		case "":
			if onContextUnavailable != nil {
				onContextUnavailable(unavailableContextMessage)
			}
		default:
			if length, ok := s.catalog.CachedContextLength(session.Context.Model); ok {
				newUsage := domainstudy.NextContextUsage(
					session.Context, session.Context.Model, session.Context.UsedTokens, length, session.Context.Estimated,
				)
				if err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
					return s.sessions.UpdateContext(ctx, sessionID, newUsage)
				}); err != nil {
					if onContextUnavailable != nil {
						onContextUnavailable(unavailableContextMessage)
					}
				} else {
					session.Context = newUsage
				}
			} else {
				s.resolveContextLengthInBackground(ctx, sessionID, session.Context.Model, session.Context, onContext, onContextUnavailable)
			}
		}
	}

	return session, SessionHistory{MessageHistoryPage: page, SummaryCoversUntilMessageID: summary.CoversUntilMessageID}, nil
}
