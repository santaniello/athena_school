package llm_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fsantaniello/athena_school/internal/platform/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GivenOllamaProviderName_WhenNewProvider_ThenReturnsNonNilProvider(t *testing.T) {
	// Given:
	name := "ollama"
	host := "http://localhost:11434"

	// When:
	provider, err := llm.NewProvider(name, host)

	// Then:
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func Test_GivenUnknownProviderName_WhenNewProvider_ThenReturnsError(t *testing.T) {
	// Given:
	name := "nonexistent"
	host := "http://localhost:11434"

	// When:
	_, err := llm.NewProvider(name, host)

	// Then:
	assert.Error(t, err)
}

func Test_GivenUnknownProviderName_WhenNewProvider_ThenErrorMessageContainsProviderName(t *testing.T) {
	// Given:
	name := "openai"
	host := "http://localhost:11434"

	// When:
	_, err := llm.NewProvider(name, host)

	// Then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai")
}

func Test_GivenOllamaProvider_WhenChat_ThenReturnsModelResponse(t *testing.T) {
	// Given:
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"llama3","message":{"role":"assistant","content":"Hello!"},"done":true}`)
	}))
	defer server.Close()
	provider, err := llm.NewProvider("ollama", server.URL)
	require.NoError(t, err)
	req := llm.ChatRequest{
		Model:    "llama3",
		Messages: []llm.ChatMessage{{Role: "user", Content: "Say hello in one word."}},
	}

	// When:
	resp, err := provider.Chat(context.Background(), req)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "Hello!", resp.Content)
}
