package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

func TestHydeEnricher_returnsTheGeneratedHypotheticalPassage(t *testing.T) {
	// Given a provider that returns a hypothetical passage
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.SessionID == "session-1" &&
				req.Task == domainllm.TaskQueryEnrichment &&
				len(req.Messages) == 2 &&
				req.Messages[1].Content != ""
		})).
		Return(domainllm.ChatResponse{Content: "Hypothetical passage about eviction policies."}, nil).
		Once()
	enricher := NewHydeEnricher(llm)

	// When enriching a query
	result, err := enricher.Enrich(context.Background(), "session-1", "Redis", "Quais são as políticas de evicção")

	// Then it returns the generated passage verbatim
	require.NoError(t, err)
	assert.Equal(t, "Hypothetical passage about eviction policies.", result)
}

func TestHydeEnricher_includesTopicAndQuestionInTheRequest(t *testing.T) {
	// Given a provider that captures the request it received
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().
		Chat(mock.Anything, mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return len(req.Messages) == 2 &&
				strings.Contains(req.Messages[1].Content, "Redis") &&
				strings.Contains(req.Messages[1].Content, "Quais são as políticas de evicção")
		})).
		Return(domainllm.ChatResponse{Content: "..."}, nil).
		Once()
	enricher := NewHydeEnricher(llm)

	// When enriching a query
	_, err := enricher.Enrich(context.Background(), "session-1", "Redis", "Quais são as políticas de evicção")

	// Then the request carried both the topic and the question
	require.NoError(t, err)
}

func TestHydeEnricher_propagatesProviderError(t *testing.T) {
	// Given a provider whose Chat call fails
	llm := llmmocks.NewMockProvider(t)
	providerErr := errors.New("rate limited")
	llm.EXPECT().Chat(mock.Anything, mock.AnythingOfType("llm.ChatRequest")).Return(domainllm.ChatResponse{}, providerErr).Once()
	enricher := NewHydeEnricher(llm)

	// When enriching a query
	_, err := enricher.Enrich(context.Background(), "session-1", "Redis", "Quais são as políticas de evicção")

	// Then the error is propagated, wrapped
	require.Error(t, err)
	assert.ErrorIs(t, err, providerErr)
}
