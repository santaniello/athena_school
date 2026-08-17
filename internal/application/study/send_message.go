package study

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// SendMessage appends the user's message to sessionID (persisted
// immediately), reloads the full history, and streams the LLM's reply via
// onChunk. topic is resent as part of a freshly built system prompt every
// turn, since the system prompt itself is never persisted as a message row.
func (s *Service) SendMessage(ctx context.Context, sessionID, topic, content string, onChunk func(chunk string) error) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrMessageRequired
	}

	userMessage := domainstudy.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domainstudy.RoleUser,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.messages.Append(ctx, userMessage); err != nil {
		return fmt.Errorf("study: appending user message: %w", err)
	}

	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("study: loading history: %w", err)
	}

	profile, err := s.profiles.Load()
	if err != nil {
		return fmt.Errorf("study: loading profile: %w", err)
	}

	llmMessages := make([]domainllm.Message, 0, len(history)+1)
	llmMessages = append(llmMessages, domainllm.Message{Role: "system", Content: buildSystemPrompt(profile, topic)})
	for _, message := range history {
		llmMessages = append(llmMessages, domainllm.Message{Role: message.Role, Content: message.Content})
	}

	if _, err := s.streamAndPersist(ctx, sessionID, llmMessages, onChunk); err != nil {
		return fmt.Errorf("study: sending message: %w", err)
	}
	return nil
}
