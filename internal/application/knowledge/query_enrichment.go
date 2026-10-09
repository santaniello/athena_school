package knowledge

import (
	"context"
	"fmt"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// hydeSystemPrompt asks the model to write a short hypothetical passage
// that would answer the question, not a reply directed at the user. This
// closes the structural length/density gap between a short
// natural-language question and the long-form chunk content it is compared
// against during retrieval — see
// specs/phases/phase-02-knowledge-engine/05-01-hyde-query-enrichment.md.
const hydeSystemPrompt = "Write a short, factual passage (2-4 sentences) that would answer the question below, as if it were an excerpt from a study note on the topic. Do not address the reader, do not say things like \"the answer is\" — write only the passage itself, in the same language as the question."

// HydeEnricher implements domainknowledge.QueryEnricher via HyDE
// (Hypothetical Document Embeddings): the generated passage is meant to be
// embedded and searched with, as an additional query alongside the raw one
// — it is never persisted or shown to the user.
type HydeEnricher struct {
	llm domainllm.Provider
}

// NewHydeEnricher creates a HydeEnricher backed by llm.
func NewHydeEnricher(llm domainllm.Provider) *HydeEnricher {
	return &HydeEnricher{llm: llm}
}

// Enrich generates the hypothetical passage for topic/content via a single,
// non-streamed, cheap-tier chat call.
func (e *HydeEnricher) Enrich(ctx context.Context, sessionID, topic, content string) (string, error) {
	resp, err := e.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskQueryEnrichment,
		Messages: []domainllm.Message{
			{Role: "system", Content: hydeSystemPrompt},
			{Role: "user", Content: fmt.Sprintf("Topic: %s\n\nQuestion: %s", topic, content)},
		},
	})
	if err != nil {
		return "", fmt.Errorf("knowledge: generating hypothetical passage: %w", err)
	}
	return resp.Content, nil
}
