package study

import (
	"context"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Resume returns sessionID's full message history, so the user can keep
// chatting in it. Never makes an LLM call and never waits on catalog I/O:
// if the session's ContextLength is unresolved (0), a cache hit is
// recomputed and persisted before this returns so the DTO already reflects
// it; a cache miss with a known model starts or joins a background refresh
// (see resolveContextLengthInBackground) without blocking the return; a
// session with no resolved model at all just surfaces the transient
// unavailable notice, since there is no trustworthy ID to refresh against.
func (s *Service) Resume(
	ctx context.Context, sessionID string,
	onContext ContextCallback, onContextUnavailable ContextUnavailableCallback,
) (domainstudy.Session, []domainstudy.Message, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: finding session: %w", err)
	}

	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return domainstudy.Session{}, nil, fmt.Errorf("study: loading history: %w", err)
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
					return domainstudy.Session{}, nil, fmt.Errorf("study: persisting resolved context: %w", err)
				}
				session.Context = newUsage
			} else {
				s.resolveContextLengthInBackground(ctx, sessionID, session.Context.Model, session.Context, onContext, onContextUnavailable)
			}
		}
	}

	return session, history, nil
}
