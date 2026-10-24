package study

import (
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// SummaryWindow reports the context window of the model currently chosen for
// conversation summaries, from the in-memory model catalog. It never does
// I/O, so it is safe on the send path; an unknown length falls back to
// domainstudy.SummaryModelWindowTokens (spec 2.31, decision 8).
type SummaryWindow struct {
	models   domainllm.ModelResolver
	contexts domainllm.ModelContextResolver
}

// NewSummaryWindow creates a SummaryWindow that resolves the summary model
// through models and its length through contexts, both on every call.
func NewSummaryWindow(models domainllm.ModelResolver, contexts domainllm.ModelContextResolver) *SummaryWindow {
	return &SummaryWindow{models: models, contexts: contexts}
}

// Tokens returns the summary model's context length, or the default when the
// catalog does not know it yet.
func (w *SummaryWindow) Tokens() int {
	model := w.models.ModelFor(domainllm.TaskConversationSummary)
	if length, ok := w.contexts.CachedContextLength(model); ok && length > 0 {
		return length
	}
	return domainstudy.SummaryModelWindowTokens
}
