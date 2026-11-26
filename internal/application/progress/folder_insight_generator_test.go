package progress

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func TestFolderInsightGenerator_asksTheModelOnceAndReturnsItsAnswerAndModel(t *testing.T) {
	// Given a provider that answers inside a Markdown fence
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	input := testFolderInsightInput()
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task: domainllm.TaskProgressInsight, Messages: buildFolderInsightPrompt(input, "pt"),
	}).Return(domainllm.ChatResponse{Content: "```json\n{\"analysis\":[]}\n```", Model: "anthropic/x"}, nil)

	// When generating a folder insight
	raw, err := NewFolderInsightGenerator(provider).GenerateFolderInsight(ctx, input, "pt")

	// Then the fence is stripped and the exact model is reported
	require.NoError(t, err)
	assert.Equal(t, `{"analysis":[]}`, raw.Content)
	assert.Equal(t, "anthropic/x", raw.Model)
}

func TestFolderInsightGenerator_aProviderErrorIsReturned(t *testing.T) {
	provider := llmmocks.NewMockProvider(t)
	boom := errors.New("provider down")
	provider.EXPECT().Chat(context.Background(), domainllm.ChatRequest{
		Task: domainllm.TaskProgressInsight, Messages: buildFolderInsightPrompt(testFolderInsightInput(), "en"),
	}).Return(domainllm.ChatResponse{}, boom)

	_, err := NewFolderInsightGenerator(provider).GenerateFolderInsight(context.Background(), testFolderInsightInput(), "en")

	assert.ErrorIs(t, err, boom)
}
