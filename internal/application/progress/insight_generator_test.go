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

func TestInsightGenerator_asksTheModelOnceAndReturnsItsAnswerAndModel(t *testing.T) {
	// Given a provider that answers inside a Markdown fence
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	input := testInsightInput()
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "s1", Task: domainllm.TaskProgressInsight, Messages: buildInsightPrompt(input, "pt"),
	}).Return(domainllm.ChatResponse{Content: "```json\n{\"analysis\":[]}\n```", Model: "anthropic/x"}, nil)

	// When generating an insight
	raw, err := NewInsightGenerator(provider).GenerateInsight(ctx, input, "pt")

	// Then the fence is stripped and the exact model is reported
	require.NoError(t, err)
	assert.Equal(t, `{"analysis":[]}`, raw.Content)
	assert.Equal(t, "anthropic/x", raw.Model)
}

func TestInsightGenerator_aProviderErrorIsReturned(t *testing.T) {
	// Given a failing provider
	provider := llmmocks.NewMockProvider(t)
	boom := errors.New("provider down")
	provider.EXPECT().Chat(context.Background(), domainllm.ChatRequest{
		SessionID: "s1", Task: domainllm.TaskProgressInsight, Messages: buildInsightPrompt(testInsightInput(), "en"),
	}).Return(domainllm.ChatResponse{}, boom)

	// When generating an insight
	_, err := NewInsightGenerator(provider).GenerateInsight(context.Background(), testInsightInput(), "en")

	// Then the cause is returned
	assert.ErrorIs(t, err, boom)
}
