package study

import (
	"context"
	"fmt"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// RequestOpeningTurn streams the assistant's opening turn for sessionID
// (about topic) via onChunk. The turn is driven purely by the system prompt
// built from the current UserProfile — there is no user message to persist
// first, since the user has only picked a topic so far.
func (s *Service) RequestOpeningTurn(ctx context.Context, sessionID, topic string, onChunk func(chunk string) error) error {
	profile, err := s.profiles.Load()
	if err != nil {
		return fmt.Errorf("study: loading profile: %w", err)
	}

	systemPrompt := buildSystemPrompt(profile, topic)
	openingTurn := []domainllm.Message{{Role: "system", Content: systemPrompt}}
	if _, err := s.streamAndPersist(ctx, sessionID, openingTurn, onChunk); err != nil {
		return fmt.Errorf("study: requesting opening turn: %w", err)
	}
	return nil
}
