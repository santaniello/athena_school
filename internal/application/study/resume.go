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
// (SourceModeWeb, a strict-notes miss, the opening turn), carries a nil
// Sources.
type MessageWithSources struct {
	domainstudy.Message
	Sources []domainknowledge.Source
}

// Resume returns sessionID's full message history, so the user can keep
// chatting in it. Never makes an LLM call and never waits on catalog I/O:
// if the session's ContextLength is unresolved (0), a cache hit is
// recomputed and persisted before this returns so the DTO already reflects
// it; a cache miss with a known model starts or joins a background refresh
// (see resolveContextLengthInBackground) without blocking the return; a
// session with no resolved model at all just surfaces the transient
// unavailable notice, since there is no trustworthy ID to refresh against.
// Resume is a read: a failure to persist a freshly resolved context length
// degrades to the transient unavailable notice instead of denying access to
// the session/history already loaded — the next real measurement persists
// it again regardless.
func (s *Service) Resume(
	ctx context.Context, sessionID string,
	onContext ContextCallback, onContextUnavailable ContextUnavailableCallback,
) (domainstudy.Session, []MessageWithSources, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: finding session: %w", err)
	}

	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: loading history: %w", err)
	}
	sourcesByMessage, err := s.messageSources.ListBySession(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: loading message sources: %w", err)
	}
	historyWithSources := make([]MessageWithSources, len(history))
	for i, message := range history {
		historyWithSources[i] = MessageWithSources{Message: message, Sources: sourcesByMessage[message.ID]}
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

	return session, historyWithSources, nil
}
