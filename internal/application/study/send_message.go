package study

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// SendMessage appends the user's message to sessionID (persisted
// immediately), retrieves local knowledge per sourceMode, and streams the
// LLM's reply via onChunk. topic is resent as part of a freshly built
// system prompt every turn, since the system prompt itself is never
// persisted as a message row.
//
// onSources is invoked exactly once per call, before any onChunk delivery:
// with the post-cap sources for a local mode that found chunks, an empty
// slice for SourceModeWeb or a local miss, or not at all if a technical
// error (invalid mode, blank content, or a retrieval failure) stops the
// turn before a response is produced.
func (s *Service) SendMessage(
	ctx context.Context, sessionID, topic, content, sourceMode string,
	onSources func([]domainknowledge.Source) error,
	onChunk func(chunk string) error,
) error {
	if !isValidSourceMode(sourceMode) {
		return domainknowledge.ErrInvalidSourceMode
	}

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

	var knowledgeMessage *domainllm.Message
	if sourceMode == domainknowledge.SourceModeWeb {
		if err := emitSources(onSources, nil); err != nil {
			return err
		}
	} else {
		result, err := s.retriever.Retrieve(ctx, sessionID, buildRetrievalQuery(topic, content))
		if err != nil {
			return fmt.Errorf("study: retrieving local knowledge: %w", err)
		}
		if err := emitSources(onSources, result.Sources); err != nil {
			return err
		}
		if len(result.Chunks) == 0 {
			if sourceMode == domainknowledge.SourceModeStrictNotes {
				return s.persistFixedStrictMissResponse(ctx, sessionID, onChunk)
			}
		} else {
			message := buildKnowledgeContext(result, sourceMode)
			knowledgeMessage = &message
		}
	}

	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("study: loading history: %w", err)
	}

	profile, err := s.profiles.Load()
	if err != nil {
		return fmt.Errorf("study: loading profile: %w", err)
	}

	llmMessages := make([]domainllm.Message, 0, len(history)+2)
	llmMessages = append(llmMessages, domainllm.Message{Role: "system", Content: buildSystemPrompt(profile, topic)})
	if knowledgeMessage != nil {
		llmMessages = append(llmMessages, *knowledgeMessage)
	}
	for _, message := range history {
		llmMessages = append(llmMessages, domainllm.Message{Role: message.Role, Content: message.Content})
	}

	if _, err := s.streamAndPersist(ctx, sessionID, llmMessages, onChunk); err != nil {
		return fmt.Errorf("study: sending message: %w", err)
	}
	return nil
}

// isValidSourceMode reports whether mode is one of the three exported
// SourceMode constants.
func isValidSourceMode(mode string) bool {
	switch mode {
	case domainknowledge.SourceModeNotes, domainknowledge.SourceModeStrictNotes, domainknowledge.SourceModeWeb:
		return true
	default:
		return false
	}
}

// buildRetrievalQuery builds the one deterministic query RAG embeds: the
// normalized session topic plus the current trimmed message, excluding
// history — no LLM call rewrites it.
func buildRetrievalQuery(topic, content string) string {
	return fmt.Sprintf("Topic: %s\n\nMessage: %s", topic, content)
}

// emitSources calls onSources with sources, normalizing a nil slice to an
// empty one so a miss/web turn always clears pending UI state with a
// concrete empty list, never nil.
func emitSources(onSources func([]domainknowledge.Source) error, sources []domainknowledge.Source) error {
	if sources == nil {
		sources = []domainknowledge.Source{}
	}
	return onSources(sources)
}

// persistFixedStrictMissResponse is strict-notes' only no-chat-call
// branch: a successful retrieval with no surviving chunks. It persists
// domainknowledge.NoLocalKnowledgeMessage as the assistant reply and
// delivers it through the normal chunk callback as one complete chunk, so
// the user question and this fixed response reappear together on resume.
func (s *Service) persistFixedStrictMissResponse(ctx context.Context, sessionID string, onChunk func(chunk string) error) error {
	assistantMessage := domainstudy.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domainstudy.RoleAssistant,
		Content:   domainknowledge.NoLocalKnowledgeMessage,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.messages.Append(ctx, assistantMessage); err != nil {
		return fmt.Errorf("study: persisting fixed strict-notes response: %w", err)
	}
	return onChunk(domainknowledge.NoLocalKnowledgeMessage)
}
