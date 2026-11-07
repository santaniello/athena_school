package architecture

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

type fakeLanguage struct {
	code string
	err  error
}

func (f fakeLanguage) ReadProfile(context.Context) (domainexercise.GenerationProfile, error) {
	return domainexercise.GenerationProfile{Language: f.code}, f.err
}

func TestHintGenerator_GenerateHint_asksTheModelOnceAndReturnsItsAnswer(t *testing.T) {
	// Given a provider that answers and a Portuguese assistant language
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	input := testHintInput()
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task: domainllm.TaskArchitectureLab, Messages: buildHintPrompt(input, "pt"),
	}).Return(domainllm.ChatResponse{Content: "E se o cache cair?"}, nil)
	generator := NewHintGenerator(provider, fakeLanguage{code: "pt"})

	// When generating a hint
	hint, err := generator.GenerateHint(ctx, input)

	// Then the model's text is returned
	require.NoError(t, err)
	assert.Equal(t, "E se o cache cair?", hint)
}

func TestHintGenerator_GenerateHint_aBlankAnswerIsAnError(t *testing.T) {
	// Given a provider that answers with only whitespace
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task: domainllm.TaskArchitectureLab, Messages: buildHintPrompt(testHintInput(), "en"),
	}).Return(domainllm.ChatResponse{Content: " \n"}, nil)

	// When generating a hint
	_, err := NewHintGenerator(provider, fakeLanguage{code: "en"}).GenerateHint(ctx, testHintInput())

	// Then no hint is delivered
	assert.ErrorIs(t, err, ErrEmptyHint)
}

func TestHintGenerator_GenerateHint_aProviderErrorIsReturned(t *testing.T) {
	// Given a failing provider
	provider := llmmocks.NewMockProvider(t)
	ctx := context.Background()
	boom := errors.New("provider down")
	provider.EXPECT().Chat(ctx, domainllm.ChatRequest{
		Task: domainllm.TaskArchitectureLab, Messages: buildHintPrompt(testHintInput(), "en"),
	}).Return(domainllm.ChatResponse{}, boom)

	// When generating a hint
	_, err := NewHintGenerator(provider, fakeLanguage{code: "en"}).GenerateHint(ctx, testHintInput())

	// Then the cause is returned
	assert.ErrorIs(t, err, boom)
}

func TestHintGenerator_GenerateHint_aProfileErrorIsReturnedWithoutCallingTheModel(t *testing.T) {
	// Given a profile that cannot be read
	boom := errors.New("no profile")
	provider := llmmocks.NewMockProvider(t)

	// When generating a hint
	_, err := NewHintGenerator(provider, fakeLanguage{err: boom}).GenerateHint(context.Background(), testHintInput())

	// Then the cause is returned and the provider was not called
	assert.ErrorIs(t, err, boom)
}
