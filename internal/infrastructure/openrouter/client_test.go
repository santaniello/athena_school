package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	"github.com/santaniello/athena/internal/domain/llm/mocks"
)

func decodeRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	return body
}

// usageEntryMatcher returns a typed mock.MatchedBy for the deterministic
// fields of a UsageEntry, while only requiring ID/CreatedAt to be set
// (they're generated per-call and not worth asserting exactly).
func usageEntryMatcher(sessionID, model string, inputTokens, outputTokens int, cost float64) any {
	return mock.MatchedBy(func(got domainllm.UsageEntry) bool {
		return got.SessionID == sessionID &&
			got.Model == model &&
			got.InputTokens == inputTokens &&
			got.OutputTokens == outputTokens &&
			got.Cost == cost &&
			got.ID != "" &&
			!got.CreatedAt.IsZero()
	})
}

// ---- Chat ----

func TestClient_Chat_returnsResponse_whenOpenRouterSucceeds(t *testing.T) {
	// Given a fake OpenRouter that accepts a chat completion request
	var receivedModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequestBody(t, r)
		receivedModel, _ = body["model"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "Hello, how can I help?"}},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25, "cost": 0.0012},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", "anthropic/claude-sonnet-4.5", 10, 15, 0.0012)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When sending a chat request for a study task
	resp, err := client.Chat(context.Background(), domainllm.ChatRequest{
		SessionID: "sess-1",
		Task:      domainllm.TaskStudy,
		Messages:  []domainllm.Message{{Role: "user", Content: "explain hexagonal architecture"}},
	})

	// Then it returns the response content and routed model, and sent the
	// tier's model in the request
	require.NoError(t, err)
	assert.Equal(t, "Hello, how can I help?", resp.Content)
	assert.Equal(t, "anthropic/claude-sonnet-4.5", resp.Model)
	assert.Equal(t, domainllm.Usage{InputTokens: 10, OutputTokens: 15, Cost: 0.0012}, resp.Usage)
	assert.False(t, resp.UsedFreeFallback)
	assert.Equal(t, "anthropic/claude-sonnet-4.5", receivedModel)
}

func TestClient_Chat_returnsErrAPIKeyMissing_whenAPIKeyIsEmpty(t *testing.T) {
	// Given a client with no API key configured
	requestReachedServer := false
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestReachedServer = true
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "", recorder)

	// When sending a chat request
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then it fails with the missing-key sentinel without reaching OpenRouter
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyMissing)
	assert.False(t, requestReachedServer)
}

func TestClient_Chat_returnsErrAPIKeyInvalid_whenOpenRouterReturns401(t *testing.T) {
	// Given a fake OpenRouter that rejects the key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-invalid", recorder)

	// When sending a chat request
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then it fails with the invalid-key sentinel
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyInvalid)
}

func TestClient_Chat_returnsGenericError_whenOpenRouterReturnsServerError(t *testing.T) {
	// Given a fake OpenRouter that errors for an unrelated reason
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When sending a chat request
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then it fails, but not with any of the specific sentinels
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domainllm.ErrAPIKeyInvalid)
	assert.NotErrorIs(t, err, domainllm.ErrInsufficientCredits)
}

func TestClient_Chat_returnsError_whenRecordingUsageFails(t *testing.T) {
	// Given a fake OpenRouter that succeeds, but a usage recorder that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "hi"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2, "cost": 0.0001},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(errors.New("disk full")).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When sending a chat request
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then the whole call fails even though the LLM responded successfully
	assert.Error(t, err)
}

func TestClient_Chat_fallsBackToFreeModel_whenFirstAttemptReturns402(t *testing.T) {
	// Given a fake OpenRouter that rejects the tier model for lack of
	// credits, but accepts the free fallback model
	var receivedModels []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequestBody(t, r)
		model, _ := body["model"].(string)
		receivedModels = append(receivedModels, model)
		if model != domainllm.FreeFallbackModel {
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "free tier answer"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10, "cost": 0},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", domainllm.FreeFallbackModel, 5, 5, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When sending a chat request for a premium task
	resp, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskInterviewEvaluation})

	// Then it retries against the free fallback model and returns that
	// response, flagged as a fallback
	require.NoError(t, err)
	assert.Equal(t, "free tier answer", resp.Content)
	assert.Equal(t, domainllm.FreeFallbackModel, resp.Model)
	assert.True(t, resp.UsedFreeFallback)
	assert.Equal(t, []string{"anthropic/claude-opus-4.5", domainllm.FreeFallbackModel}, receivedModels)
}

func TestClient_Chat_returnsErrInsufficientCredits_whenFallbackAlsoReturns402(t *testing.T) {
	// Given a fake OpenRouter that rejects both the tier model and the free
	// fallback model for lack of credits
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When sending a chat request
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then it fails with the insufficient-credits sentinel after exactly one
	// retry, never a third attempt
	assert.ErrorIs(t, err, domainllm.ErrInsufficientCredits)
	assert.Equal(t, 2, callCount)
}

// ---- ChatStream ----

func sseFrame(w http.ResponseWriter, data string) {
	_, _ = w.Write([]byte("data: " + data + "\n\n"))
	w.(http.Flusher).Flush()
}

func TestClient_ChatStream_deliversChunksToHandlerInOrder(t *testing.T) {
	// Given a fake OpenRouter streaming two content chunks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[{"delta":{"content":"Hel"}}]}`)
		sseFrame(w, `{"choices":[{"delta":{"content":"lo"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	var received []string
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	// Then the handler receives each chunk in order and nothing else
	require.NoError(t, err)
	assert.Equal(t, []string{"Hel", "lo"}, received)
}

func TestClient_ChatStream_recordsUsageOnce_fromTheLastUsageBearingFrame(t *testing.T) {
	// Given a stream whose final frame carries usage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `{"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10,"cost":0.002}}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	ctx := context.Background()
	// No frame carries a "model" field either, so the resolved model is
	// unconfirmed — recorded empty rather than under the requested alias.
	recorder.EXPECT().Record(ctx, usageEntryMatcher("sess-1", "", 7, 3, 0.002)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(ctx, domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then usage is recorded once, using the last usage-bearing frame
	require.NoError(t, err)
}

func TestClient_ChatStream_recordsZeroUsage_whenNoFrameCarriesUsage(t *testing.T) {
	// Given a stream where no frame carries usage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	ctx := context.Background()
	// No frame carries a "model" field either, so the resolved model is
	// unconfirmed — recorded empty rather than under the requested alias.
	recorder.EXPECT().Record(ctx, usageEntryMatcher("sess-1", "", 0, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(ctx, domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then usage is still recorded once, zeroed out
	require.NoError(t, err)
}

func TestClient_ChatStream_stopsCallingHandler_whenHandlerReturnsError(t *testing.T) {
	// Given a stream with two chunks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[{"delta":{"content":"Hel"}}]}`)
		sseFrame(w, `{"choices":[{"delta":{"content":"lo"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)
	handlerErr := errors.New("ui closed")

	// When the handler fails on the first chunk
	callCount := 0
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error {
		callCount++
		return handlerErr
	})

	// Then ChatStream returns that error, the handler is called exactly
	// once, and usage is never recorded for the aborted stream
	assert.ErrorIs(t, err, handlerErr)
	assert.Equal(t, 1, callCount)
}

func TestClient_ChatStream_skipsFrameWithEmptyChoices(t *testing.T) {
	// Given a stream with a frame that has no choices
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[]}`)
		sseFrame(w, `{"choices":[{"delta":{"content":"ok"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	var received []string
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	// Then it does not panic and only calls the handler for real content
	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, received)
}

func TestClient_ChatStream_returnsError_whenChunkIsNotValidJSON(t *testing.T) {
	// Given a stream with a malformed data frame
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{not valid json`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then it fails loudly instead of silently skipping the bad frame
	assert.Error(t, err)
}

func TestClient_ChatStream_returnsErrAPIKeyMissing_whenAPIKeyIsEmpty(t *testing.T) {
	// Given a client with no API key configured
	requestReachedServer := false
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestReachedServer = true
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then it fails with the missing-key sentinel without reaching OpenRouter
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyMissing)
	assert.False(t, requestReachedServer)
}

func TestClient_ChatStream_returnsErrAPIKeyInvalid_whenOpenRouterReturns401(t *testing.T) {
	// Given a fake OpenRouter that rejects the key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-invalid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then it fails with the invalid-key sentinel
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyInvalid)
}

func TestClient_ChatStream_returnsGenericError_whenOpenRouterReturnsServerError(t *testing.T) {
	// Given a fake OpenRouter that errors for an unrelated reason
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then it fails, but not with any of the specific sentinels
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domainllm.ErrAPIKeyInvalid)
	assert.NotErrorIs(t, err, domainllm.ErrInsufficientCredits)
}

func TestClient_ChatStream_returnsError_whenRecordingUsageFails(t *testing.T) {
	// Given a fake OpenRouter that streams successfully, but a usage
	// recorder that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(errors.New("disk full")).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then the whole call fails even though the stream completed
	assert.Error(t, err)
}

func TestClient_ChatStream_fallsBackToFreeModel_whenFirstAttemptReturns402(t *testing.T) {
	// Given a fake OpenRouter that rejects the tier model for lack of
	// credits, but streams successfully for the free fallback model
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequestBody(t, r)
		model, _ := body["model"].(string)
		if model != domainllm.FreeFallbackModel {
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}
		sseFrame(w, `{"choices":[{"delta":{"content":"free"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	ctx := context.Background()
	// No frame carries a "model" field either, so the resolved model is
	// unconfirmed — recorded empty rather than under the free-fallback alias.
	recorder.EXPECT().Record(ctx, usageEntryMatcher("sess-1", "", 0, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request for a premium task
	var received []string
	_, err := client.ChatStream(ctx, domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskInterviewEvaluation}, func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	// Then the handler only ever receives chunks from the successful,
	// retried attempt
	require.NoError(t, err)
	assert.Equal(t, []string{"free"}, received)
}

func TestClient_ChatStream_returnsErrInsufficientCredits_whenFallbackAlsoReturns402(t *testing.T) {
	// Given a fake OpenRouter that rejects both the tier model and the free
	// fallback model for lack of credits
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	_, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then it fails with the insufficient-credits sentinel after exactly one
	// retry, never a third attempt
	assert.ErrorIs(t, err, domainllm.ErrInsufficientCredits)
	assert.Equal(t, 2, callCount)
}

func TestClient_ChatStream_resolvesModel_whenEveryNonEmptyFrameAgrees(t *testing.T) {
	// Given a stream whose frames all report the same concrete model —
	// including a frame that omits it, which must be ignored rather than
	// treated as a conflict
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"model":"anthropic/claude-sonnet-4.5","choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `{"choices":[{"delta":{"content":" there"}}]}`)
		sseFrame(w, `{"model":"anthropic/claude-sonnet-4.5","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", "anthropic/claude-sonnet-4.5", 1, 1, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	resp, err := client.ChatStream(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then the resolved model is reported on the response and used to record
	// usage
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-sonnet-4.5", resp.Model)
}

func TestClient_ChatStream_leavesModelEmpty_whenFramesConflict(t *testing.T) {
	// Given a stream whose frames disagree on the resolved model — must not
	// invent one
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"model":"provider/model-a","choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `{"model":"provider/model-b","choices":[{"delta":{"content":" there"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	ctx := context.Background()
	// Recorded under an empty model too — conflicting metadata means the
	// resolved model is unconfirmed, so usage must not be attributed to
	// the requested alias as if it were confirmed.
	recorder.EXPECT().Record(ctx, usageEntryMatcher("sess-1", "", 0, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	resp, err := client.ChatStream(ctx, domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then the resolved model is left empty rather than guessed
	require.NoError(t, err)
	assert.Empty(t, resp.Model)
}

func TestClient_ChatStream_leavesModelEmpty_whenNoFrameReportsOne(t *testing.T) {
	// Given a stream where no frame carries a "model" field at all
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sseFrame(w, `{"choices":[{"delta":{"content":"hi"}}]}`)
		sseFrame(w, `[DONE]`)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	ctx := context.Background()
	recorder.EXPECT().Record(ctx, usageEntryMatcher("sess-1", "", 0, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When streaming a chat request
	resp, err := client.ChatStream(ctx, domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy}, func(string) error { return nil })

	// Then the resolved model stays empty
	require.NoError(t, err)
	assert.Empty(t, resp.Model)
}

// ---- ListModels ----

func TestClient_ListModels_returnsModelInfoFromCatalogResponse(t *testing.T) {
	// Given a fake OpenRouter model catalog with two entries
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "anthropic/claude-sonnet-4.5", "context_length": 200000},
				{"id": "openrouter/free", "context_length": 32000},
			},
		})
	}))
	defer server.Close()
	client := NewClient(server.URL, "sk-or-valid", mocks.NewMockUsageRecorder(t))

	// When listing models
	models, err := client.ListModels(context.Background())

	// Then it returns every entry, unvalidated (validation is the caller's
	// job)
	require.NoError(t, err)
	assert.Equal(t, []domainllm.ModelInfo{
		{ID: "anthropic/claude-sonnet-4.5", ContextLength: 200000},
		{ID: "openrouter/free", ContextLength: 32000},
	}, models)
}

func TestClient_ListModels_worksWithoutAnAPIKey(t *testing.T) {
	// Given a client with no API key configured — the catalog can load
	// before onboarding sets one
	var receivedAuth string
	authHeaderSet := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		authHeaderSet = receivedAuth != ""
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()
	client := NewClient(server.URL, "", mocks.NewMockUsageRecorder(t))

	// When listing models
	_, err := client.ListModels(context.Background())

	// Then it succeeds without sending an Authorization header
	require.NoError(t, err)
	assert.False(t, authHeaderSet, "expected no Authorization header, got %q", receivedAuth)
}

func TestClient_ListModels_returnsError_whenOpenRouterReturnsServerError(t *testing.T) {
	// Given a fake OpenRouter that errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := NewClient(server.URL, "sk-or-valid", mocks.NewMockUsageRecorder(t))

	// When listing models
	_, err := client.ListModels(context.Background())

	// Then it fails
	assert.Error(t, err)
}

// ---- Embeddings ----

func TestClient_Embeddings_returnsFloatSlice_whenOpenRouterSucceeds(t *testing.T) {
	// Given a fake OpenRouter that accepts an embeddings request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  []map[string]any{{"embedding": []float64{0.1, 0.2, 0.3}, "index": 0}},
			"usage": map[string]any{"prompt_tokens": 4, "total_tokens": 4},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, usageEntryMatcher("sess-1", domainllm.EmbeddingModel, 4, 0, 0)).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When requesting embeddings
	resp, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "hexagonal architecture"})

	// Then it returns the float slice and the fixed embedding model, with
	// cost defaulting to zero since OpenRouter's embeddings usage omits it
	require.NoError(t, err)
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, resp.Embedding)
	assert.Equal(t, domainllm.EmbeddingModel, resp.Model)
	assert.Equal(t, domainllm.Usage{InputTokens: 4, OutputTokens: 0, Cost: 0}, resp.Usage)
}

func TestClient_Embeddings_sendsInputAndEmbeddingModelInRequestBody(t *testing.T) {
	// Given a fake OpenRouter that echoes the request body back
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  []map[string]any{{"embedding": []float64{0.1}, "index": 0}},
			"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When requesting embeddings
	_, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "some text"})

	// Then the request carries the fixed embedding model and the input text
	require.NoError(t, err)
	assert.Equal(t, domainllm.EmbeddingModel, receivedBody["model"])
	assert.Equal(t, "some text", receivedBody["input"])
}

func TestClient_Embeddings_returnsErrAPIKeyMissing_whenAPIKeyIsEmpty(t *testing.T) {
	// Given a client with no API key configured
	requestReachedServer := false
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestReachedServer = true
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "", recorder)

	// When requesting embeddings
	_, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "text"})

	// Then it fails with the missing-key sentinel without reaching OpenRouter
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyMissing)
	assert.False(t, requestReachedServer)
}

func TestClient_Embeddings_returnsErrAPIKeyInvalid_whenOpenRouterReturns401(t *testing.T) {
	// Given a fake OpenRouter that rejects the key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-invalid", recorder)

	// When requesting embeddings
	_, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "text"})

	// Then it fails with the invalid-key sentinel
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyInvalid)
}

func TestClient_Embeddings_returnsErrInsufficientCredits_whenOpenRouterReturns402(t *testing.T) {
	// Given a fake OpenRouter that rejects for lack of credits — embeddings
	// have no free-model fallback
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When requesting embeddings
	_, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "text"})

	// Then it fails with the insufficient-credits sentinel directly
	assert.ErrorIs(t, err, domainllm.ErrInsufficientCredits)
}

func TestClient_Embeddings_returnsGenericError_whenOpenRouterReturnsServerError(t *testing.T) {
	// Given a fake OpenRouter that errors for an unrelated reason
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When requesting embeddings
	_, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "text"})

	// Then it fails, but not with any of the specific sentinels
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domainllm.ErrAPIKeyInvalid)
	assert.NotErrorIs(t, err, domainllm.ErrInsufficientCredits)
}

func TestClient_Embeddings_returnsError_whenRecordingUsageFails(t *testing.T) {
	// Given a fake OpenRouter that succeeds, but a usage recorder that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  []map[string]any{{"embedding": []float64{0.1}, "index": 0}},
			"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(errors.New("disk full")).Once()
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When requesting embeddings
	_, err := client.Embeddings(context.Background(), domainllm.EmbeddingRequest{SessionID: "sess-1", Input: "text"})

	// Then the whole call fails even though the embedding was generated
	assert.Error(t, err)
}

// ---- SetAPIKey ----

func TestClient_SetAPIKey_changesKeyUsedByLaterCalls(t *testing.T) {
	// Given a client constructed with an initial key
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": "hi"}}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	recorder.EXPECT().Record(mock.Anything, mock.AnythingOfType("llm.UsageEntry")).Return(nil).Once()
	client := NewClient(server.URL, "sk-or-old", recorder)

	// When the key is rotated and a call is made afterwards
	client.SetAPIKey("sk-or-new")
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then the call authenticates with the new key
	require.NoError(t, err)
	assert.Equal(t, "Bearer sk-or-new", receivedAuth)
}

func TestClient_Chat_returnsErrAPIKeyMissing_afterSetAPIKeyClearsKey(t *testing.T) {
	// Given a client constructed with a valid key
	requestReachedServer := false
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestReachedServer = true
	}))
	defer server.Close()
	recorder := mocks.NewMockUsageRecorder(t)
	client := NewClient(server.URL, "sk-or-valid", recorder)

	// When the key is cleared and a call is made afterwards
	client.SetAPIKey("")
	_, err := client.Chat(context.Background(), domainllm.ChatRequest{SessionID: "sess-1", Task: domainllm.TaskStudy})

	// Then it fails with the missing-key sentinel without reaching OpenRouter
	assert.ErrorIs(t, err, domainllm.ErrAPIKeyMissing)
	assert.False(t, requestReachedServer)
}
