package study

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// Summarizer folds study messages into the session's incremental
// conversation summary (spec 2.24). It only produces the new summary: it does
// not decide which messages to fold nor persist the result, so a failure
// anywhere leaves the previously stored summary untouched.
type Summarizer struct {
	llm      domainllm.Provider
	profiles domainprofile.Store

	inputBudget  int
	targetTokens int
	now          func() time.Time
}

// NewSummarizer creates a Summarizer that calls llm and reads the assistant
// language from profiles.
func NewSummarizer(llm domainllm.Provider, profiles domainprofile.Store) *Summarizer {
	return &Summarizer{
		llm:          llm,
		profiles:     profiles,
		inputBudget:  domainstudy.SummaryInputBudgetTokens,
		targetTokens: domainstudy.SummaryTargetTokens(domainstudy.SummaryModelWindowTokens),
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// Summarize returns the summary of sessionID after folding messages into
// previous (nil for the first one). Messages are folded in sequential batches
// that each fit the input budget; the last folded message becomes
// CoversUntilMessageID. A summary over the hard cap gets one condensing pass,
// and ErrSummaryTooLarge if that is not enough. It returns
// ErrNoMessagesToSummarize for an empty list.
func (s *Summarizer) Summarize(ctx context.Context, sessionID string, previous *domainstudy.ConversationSummary, messages []domainstudy.Message) (domainstudy.ConversationSummary, error) {
	if len(messages) == 0 {
		return domainstudy.ConversationSummary{}, ErrNoMessagesToSummarize
	}
	profile, err := s.profiles.Load()
	if err != nil {
		return domainstudy.ConversationSummary{}, fmt.Errorf("study: loading profile: %w", err)
	}

	content := ""
	if previous != nil {
		content = previous.Content
	}
	lastID := ""
	for _, b := range splitIntoBatches(messages, s.inputBudget) {
		folded, err := s.chat(ctx, sessionID, buildSummarySystemPrompt(profile.AssistantLanguage, s.targetTokens), buildFoldUserPrompt(content, b.transcript))
		if err != nil {
			return domainstudy.ConversationSummary{}, err
		}
		content, err = s.enforceCap(ctx, sessionID, profile.AssistantLanguage, folded)
		if err != nil {
			return domainstudy.ConversationSummary{}, err
		}
		lastID = b.lastID
	}

	return domainstudy.NewConversationSummary(sessionID, content, lastID, domainstudy.EstimateTokens(content), s.now())
}

// enforceCap returns content when it fits the hard cap; otherwise it runs one
// condensing pass on the summary alone and fails with ErrSummaryTooLarge if
// the result still does not fit.
func (s *Summarizer) enforceCap(ctx context.Context, sessionID, language, content string) (string, error) {
	hardCap := domainstudy.SummaryHardCapTokens(s.targetTokens)
	if domainstudy.EstimateTokens(content) <= hardCap {
		return content, nil
	}
	condensed, err := s.chat(ctx, sessionID, buildCondensePrompt(language, s.targetTokens), buildCondenseUserPrompt(content))
	if err != nil {
		return "", err
	}
	if domainstudy.EstimateTokens(condensed) > hardCap {
		return "", ErrSummaryTooLarge
	}
	return condensed, nil
}

// chat runs one non-streamed summarization call and returns the trimmed,
// non-blank reply.
func (s *Summarizer) chat(ctx context.Context, sessionID, system, user string) (string, error) {
	resp, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskConversationSummary,
		Messages: []domainllm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", fmt.Errorf("study: summarizing conversation: %w", err)
	}
	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return "", errors.New("study: summarizing conversation: empty response")
	}
	return content, nil
}
