package progress

import (
	"context"
	"fmt"

	"github.com/santaniello/athena/internal/application/llmprompt"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// InsightGenerator implements domainprogress.InsightGenerator with one model
// call, through the provider that records its usage. It never retries.
type InsightGenerator struct {
	llm domainllm.Provider
}

// NewInsightGenerator creates an InsightGenerator backed by the given provider.
func NewInsightGenerator(llm domainllm.Provider) *InsightGenerator {
	return &InsightGenerator{llm: llm}
}

// GenerateInsight asks the model for the insight of the topic, in the given
// assistant language. The answer is returned as the JSON text, without the
// Markdown fence some models add; validating it is the domain's job.
func (g *InsightGenerator) GenerateInsight(ctx context.Context, in domainprogress.InsightInput, language string) (domainprogress.RawInsight, error) {
	response, err := g.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: in.SessionID,
		Task:      domainllm.TaskProgressInsight,
		Messages:  buildInsightPrompt(in, language),
	})
	if err != nil {
		return domainprogress.RawInsight{}, fmt.Errorf("progress: generating insight: %w", err)
	}
	return domainprogress.RawInsight{Content: llmprompt.StripCodeFence(response.Content), Model: response.Model}, nil
}
