package progress

import (
	"context"
	"fmt"

	"github.com/santaniello/athena/internal/application/llmprompt"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// FolderInsightGenerator implements domainprogress.FolderInsightGenerator with
// one model call, through the provider that records its usage. It never
// retries, and the call belongs to no session.
type FolderInsightGenerator struct {
	llm domainllm.Provider
}

// NewFolderInsightGenerator creates a FolderInsightGenerator backed by the given provider.
func NewFolderInsightGenerator(llm domainllm.Provider) *FolderInsightGenerator {
	return &FolderInsightGenerator{llm: llm}
}

// GenerateFolderInsight asks the model for the insight of the folder, in the
// given assistant language. The answer is returned as the JSON text, without
// the Markdown fence some models add; validating it is the domain's job.
func (g *FolderInsightGenerator) GenerateFolderInsight(ctx context.Context, in domainprogress.FolderInsightInput, language string) (domainprogress.RawInsight, error) {
	response, err := g.llm.Chat(ctx, domainllm.ChatRequest{
		Task:     domainllm.TaskProgressInsight,
		Messages: buildFolderInsightPrompt(in, language),
	})
	if err != nil {
		return domainprogress.RawInsight{}, fmt.Errorf("progress: generating folder insight: %w", err)
	}
	return domainprogress.RawInsight{Content: llmprompt.StripCodeFence(response.Content), Model: response.Model}, nil
}
