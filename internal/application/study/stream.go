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

// streamAndPersist sends messages to the LLM, forwarding every chunk to
// onChunk as it arrives. Only once the stream completes successfully is the
// full assistant reply persisted as a single message — never partial
// content, so a mid-stream failure leaves no assistant row (the user's turn
// it replied to, if any, is already safely persisted).
func (s *Service) streamAndPersist(ctx context.Context, sessionID string, messages []domainllm.Message, onChunk func(chunk string) error) (string, error) {
	var buf strings.Builder
	err := s.llm.ChatStream(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskStudy,
		Messages:  messages,
	}, func(chunk string) error {
		buf.WriteString(chunk)
		return onChunk(chunk)
	})
	if err != nil {
		return "", err
	}

	assistantMessage := domainstudy.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domainstudy.RoleAssistant,
		Content:   buf.String(),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.messages.Append(ctx, assistantMessage); err != nil {
		return "", fmt.Errorf("study: persisting assistant reply: %w", err)
	}
	return assistantMessage.Content, nil
}
