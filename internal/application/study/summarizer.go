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

	// window reports the window of the model that writes summaries; nil means
	// the default. It sizes the input batches and the summary target.
	window SummaryWindowSource
	// inputBudget and targetTokens, when positive, override the sizes derived
	// from window. Zero (the default) means derive them on every call, so a
	// change of summary model applies to the next summary.
	inputBudget  int
	targetTokens int
	now          func() time.Time
}

// SummaryWindowSource reports the context window, in tokens, of the model that
// writes conversation summaries. Implemented by SummaryWindow.
type SummaryWindowSource interface {
	Tokens() int
}

// WithSummaryModelWindow makes the summarizer size its input and target from
// window instead of the default 128k.
func WithSummaryModelWindow(window SummaryWindowSource) func(*Summarizer) {
	return func(s *Summarizer) { s.window = window }
}

// NewSummarizer creates a Summarizer that calls llm and reads the assistant
// language from profiles.
func NewSummarizer(llm domainllm.Provider, profiles domainprofile.Store, options ...func(*Summarizer)) *Summarizer {
	s := &Summarizer{
		llm:      llm,
		profiles: profiles,
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		option(s)
	}
	return s
}

// sizing returns the input budget and summary target for one Summarize call.
func (s *Summarizer) sizing() (inputBudget, targetTokens int) {
	window := domainstudy.SummaryModelWindowTokens
	if s.window != nil {
		window = s.window.Tokens()
	}
	inputBudget, targetTokens = s.inputBudget, s.targetTokens
	if inputBudget <= 0 {
		inputBudget = domainstudy.SummaryInputBudgetFor(window)
	}
	if targetTokens <= 0 {
		targetTokens = domainstudy.SummaryTargetTokens(window)
	}
	return inputBudget, targetTokens
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
	if err := validateSummaryInput(sessionID, previous, messages); err != nil {
		return domainstudy.ConversationSummary{}, err
	}
	profile, err := s.profiles.Load()
	if err != nil {
		return domainstudy.ConversationSummary{}, fmt.Errorf("study: loading profile: %w", err)
	}

	inputBudget, targetTokens := s.sizing()
	content := ""
	if previous != nil {
		content = previous.Content
	}
	lastID := ""
	for _, b := range splitIntoBatches(messages, inputBudget) {
		folded, err := s.chat(ctx, sessionID, buildSummarySystemPrompt(profile.AssistantLanguage, targetTokens), buildFoldUserPrompt(content, b.transcript))
		if err != nil {
			return domainstudy.ConversationSummary{}, err
		}
		if err := validateSummaryFormat(folded); err != nil {
			return domainstudy.ConversationSummary{}, err
		}
		content, err = s.enforceCap(ctx, sessionID, profile.AssistantLanguage, targetTokens, folded)
		if err != nil {
			return domainstudy.ConversationSummary{}, err
		}
		lastID = b.lastID
	}

	return domainstudy.NewConversationSummary(sessionID, content, lastID, domainstudy.EstimateTokens(content), s.now())
}

// validateSummaryInput rejects a previous summary or messages that are not
// the session's own, so a summary can never mix memory across sessions.
func validateSummaryInput(sessionID string, previous *domainstudy.ConversationSummary, messages []domainstudy.Message) error {
	if sessionID == "" {
		return fmt.Errorf("%w: empty session id", ErrInvalidSummaryInput)
	}
	if previous != nil && previous.SessionID != sessionID {
		return fmt.Errorf("%w: previous summary belongs to session %q", ErrInvalidSummaryInput, previous.SessionID)
	}
	for _, m := range messages {
		if m.ID == "" || m.SessionID != sessionID {
			return fmt.Errorf("%w: message %q belongs to session %q", ErrInvalidSummaryInput, m.ID, m.SessionID)
		}
	}
	return nil
}

// enforceCap returns content when it fits the hard cap; otherwise it runs one
// condensing pass on the summary alone and fails with ErrSummaryTooLarge if
// the result still does not fit.
func (s *Summarizer) enforceCap(ctx context.Context, sessionID, language string, targetTokens int, content string) (string, error) {
	hardCap := domainstudy.SummaryHardCapTokens(targetTokens)
	if domainstudy.EstimateTokens(content) <= hardCap {
		return content, nil
	}
	condensed, err := s.chat(ctx, sessionID, buildCondensePrompt(language, targetTokens), buildCondenseUserPrompt(content))
	if err != nil {
		return "", err
	}
	if err := validateSummaryFormat(condensed); err != nil {
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
