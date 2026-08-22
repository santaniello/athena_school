package study

import (
	"context"
	"fmt"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// RequestOpeningTurn streams the assistant's opening turn for sessionID
// (about topic) via onChunk. The turn is driven purely by the system prompt
// built from the current UserProfile — there is no user message to persist
// first, since the user has only picked a topic so far.
//
// Guards against generating a second, independent opening turn if one was
// already persisted for this session (e.g. a duplicate request racing the
// first one) by replaying the existing message instead of calling the LLM
// again. If no opening message exists yet, it observes the same in-flight
// and persisted-blocked guards as SendMessage before generating one — a
// fresh session's context always starts normal/unmeasured, so in practice
// this only matters for a session that somehow reached blocked with no
// messages at all.
func (s *Service) RequestOpeningTurn(
	ctx context.Context, sessionID, topic string,
	onChunk func(chunk string) error,
	onContext ContextCallback,
	onContextUnavailable ContextUnavailableCallback,
) error {
	existing, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("study: loading history: %w", err)
	}
	if len(existing) > 0 {
		return onChunk(existing[0].Content)
	}

	if err := s.inFlight.begin(sessionID); err != nil {
		return err
	}
	defer s.inFlight.end(sessionID)

	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("study: finding session: %w", err)
	}
	if session.Context.State == domainstudy.ContextStateBlocked {
		return domainstudy.ErrSessionContextLimitReached
	}

	profile, err := s.profiles.Load()
	if err != nil {
		return fmt.Errorf("study: loading profile: %w", err)
	}

	systemPrompt := buildSystemPrompt(profile, topic)
	openingTurn := []domainllm.Message{{Role: "system", Content: systemPrompt}}
	if _, err := s.streamAndPersist(ctx, sessionID, session.Context, openingTurn, onChunk, onContext, onContextUnavailable); err != nil {
		return fmt.Errorf("study: requesting opening turn: %w", err)
	}
	return nil
}
