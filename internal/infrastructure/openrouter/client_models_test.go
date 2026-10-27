package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	"github.com/santaniello/athena/internal/domain/llm/mocks"
)

func configuredResolver() *domainllm.OverridableResolver {
	resolver := domainllm.NewOverridableResolver()
	resolver.SetOverrides(map[domainllm.TaskType]string{domainllm.TaskStudy: "openai/gpt-5"}, "openai/text-embedding-3-large")
	return resolver
}

func chatSuccessServer(t *testing.T, receivedModel *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*receivedModel, _ = decodeRequestBody(t, r)["model"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "ok"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
}

func TestClient_Chat_sendsAndRecordsTheConfiguredModel(t *testing.T) {
	// Given a client whose resolver maps the study task to a chosen model
	var receivedModel string
	server := chatSuccessServer(t, &receivedModel)
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", "openai/gpt-5", 1, 1, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(configuredResolver()))

	// When sending a study chat request
	resp, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then the chosen model is sent, reported and recorded in usage
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-5", receivedModel)
	assert.Equal(t, "openai/gpt-5", resp.Model)
}

func TestClient_Chat_otherTasksKeepTheirTierModelWhenOneIsConfigured(t *testing.T) {
	// Given the same configured resolver
	var receivedModel string
	server := chatSuccessServer(t, &receivedModel)
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", "openai/gpt-4o-mini", 1, 1, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(configuredResolver()))

	// When sending a request for a task with no override
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskConversationSummary})

	// Then it still uses its tier default
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-4o-mini", receivedModel)
}

func TestClient_Chat_seesAResolverChangeOnTheNextCallWithoutRebuilding(t *testing.T) {
	// Given a client that already served one call
	var receivedModel string
	server := chatSuccessServer(t, &receivedModel)
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("", "anthropic/claude-sonnet-4.5", 1, 1, 0)).Return(nil).Once()
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("", "openai/gpt-5", 1, 1, 0)).Return(nil).Once()
	resolver := domainllm.NewOverridableResolver()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(resolver))
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{Task: domainllm.TaskStudy})
	require.NoError(t, err)
	require.Equal(t, "anthropic/claude-sonnet-4.5", receivedModel)

	// When the user picks another model and the next call is made
	resolver.SetOverrides(map[domainllm.TaskType]string{domainllm.TaskStudy: "openai/gpt-5"}, "")
	_, err = client.Chat(context.Background(), domainllm.ChatRequest{Task: domainllm.TaskStudy})

	// Then the new model is used
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-5", receivedModel)
}

func TestClient_Chat_withoutAResolverUsesTheTierDefault(t *testing.T) {
	// Given a client built the old way, with no resolver option
	var receivedModel string
	server := chatSuccessServer(t, &receivedModel)
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("", "anthropic/claude-sonnet-4.5", 1, 1, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When sending a study request
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{Task: domainllm.TaskStudy})

	// Then the tier default is sent
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4.5", receivedModel)
}

func TestClient_Chat_freeFallbackStillAppliesWhateverModelWasChosen(t *testing.T) {
	// Given a chosen study model and an account out of credits on the first try
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model, _ := decodeRequestBody(t, r)["model"].(string)
		models = append(models, model)
		if len(models) == 1 {
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("", domainllm.FreeFallbackModel, 0, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(configuredResolver()))

	// When sending a study request
	resp, err := client.Chat(context.Background(), domainllm.ChatRequest{Task: domainllm.TaskStudy})

	// Then the retry goes to the free router, not the chosen model
	require.NoError(t, err)
	assert.Equal(t, []string{"openai/gpt-5", domainllm.FreeFallbackModel}, models)
	assert.True(t, resp.UsedFreeFallback)
}

func TestClient_ChatStream_sendsTheConfiguredModel(t *testing.T) {
	// Given a streaming server that records the requested model
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedModel, _ = decodeRequestBody(t, r)["model"].(string)
		sseFrame(w, `{"model":"openai/gpt-5","choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", "openai/gpt-5", 0, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(configuredResolver()))

	// When streaming a study request
	resp, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then the chosen model is requested and reported
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-5", receivedModel)
	assert.Equal(t, "openai/gpt-5", resp.Model)
}

func TestClient_ChatStream_inFlightStreamKeepsItsModelWhenSettingsChange(t *testing.T) {
	// Given a stream that is still open when the user changes the study model
	var receivedModel string
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedModel, _ = decodeRequestBody(t, r)["model"].(string)
		sseFrame(w, `{"model":"anthropic/claude-sonnet-4.5","choices":[{"delta":{"content":"a"}}]}`)
		close(started)
		<-release
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("", "anthropic/claude-sonnet-4.5", 0, 0, 0)).Return(nil).Once()
	resolver := domainllm.NewOverridableResolver()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(resolver))
	done := make(chan error, 1)
	go func() {
		_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{Task: domainllm.TaskStudy}, func(string) error { return nil })
		done <- err
	}()
	<-started

	// When the setting changes mid-stream
	resolver.SetOverrides(map[domainllm.TaskType]string{domainllm.TaskStudy: "openai/gpt-5"}, "")
	close(release)

	// Then the running stream was already bound to the model it started with
	require.NoError(t, <-done)
	assert.Equal(t, "anthropic/claude-sonnet-4.5", receivedModel)
}

func embeddingsServer(t *testing.T, receivedModel *string, count int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*receivedModel, _ = decodeRequestBody(t, r)["model"].(string)
		data := make([]map[string]any, count)
		for i := range data {
			data[i] = map[string]any{"embedding": []float64{0.1}, "index": i}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "usage": map[string]any{"prompt_tokens": 1}})
	}))
}

func TestClient_Embeddings_usesTheResolversEmbeddingModel(t *testing.T) {
	// Given a resolver with a chosen embedding model
	var receivedModel string
	server := embeddingsServer(t, &receivedModel, 1)
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("s", "openai/text-embedding-3-large", 1, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(configuredResolver()))

	// When embedding a text without naming a model
	resp, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "s", Input: "x"})

	// Then the chosen model is sent, recorded and reported
	require.NoError(t, err)
	assert.Equal(t, "openai/text-embedding-3-large", receivedModel)
	assert.Equal(t, "openai/text-embedding-3-large", resp.Model)
}

func TestClient_EmbeddingsBatch_usesTheResolversEmbeddingModel(t *testing.T) {
	// Given a resolver with a chosen embedding model
	var receivedModel string
	server := embeddingsServer(t, &receivedModel, 2)
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("s", "openai/text-embedding-3-large", 1, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder, WithModelResolver(configuredResolver()))

	// When embedding a batch
	resp, err := client.EmbeddingsBatch(context.Background(), domainllm.EmbeddingBatchRequest{SessionID: "s", Inputs: []string{"a", "b"}})

	// Then the chosen model is sent, recorded and reported
	require.NoError(t, err)
	assert.Equal(t, "openai/text-embedding-3-large", receivedModel)
	assert.Equal(t, "openai/text-embedding-3-large", resp.Model)
}

func TestClient_ListEmbeddingModels_readsTheEmbeddingsCatalog(t *testing.T) {
	// Given a fake OpenRouter whose embeddings catalog is a separate endpoint
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "openai/text-embedding-3-small", "context_length": 8192},
			{"id": "openai/text-embedding-3-large", "context_length": 8192},
		}})
	}))
	defer server.Close()
	client := NewClient(server.URL, "", mocks.NewMockUsageRecorder(t))

	// When listing embedding models
	models, err := client.ListEmbeddingModels(context.Background())

	// Then it returns them from /api/v1/embeddings/models without an API key
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/embeddings/models", path)
	assert.Equal(t, []domainllm.ModelInfo{
		{ID: "openai/text-embedding-3-small", ContextLength: 8192},
		{ID: "openai/text-embedding-3-large", ContextLength: 8192},
	}, models)
}

func TestClient_ListEmbeddingModels_returnsAnError_whenOpenRouterFails(t *testing.T) {
	// Given a fake OpenRouter that errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := NewClient(server.URL, "", mocks.NewMockUsageRecorder(t))

	// When listing embedding models
	_, err := client.ListEmbeddingModels(context.Background())

	// Then it fails
	assert.Error(t, err)
}
