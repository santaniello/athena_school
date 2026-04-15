package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func nonStreamingHandler(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"llama3","message":{"role":"assistant","content":%q},"done":true}`, content)
	}
}

func streamingHandler(tokens []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		for _, token := range tokens {
			fmt.Fprintf(w, `{"model":"llama3","message":{"role":"assistant","content":%q},"done":false}`+"\n", token)
			flusher.Flush()
		}
		fmt.Fprintln(w, `{"model":"llama3","message":{"role":"assistant","content":""},"done":true}`)
		flusher.Flush()
	}
}

func baseRequest() Request {
	return Request{
		Model:    "llama3",
		Messages: []Message{{Role: "user", Content: "Say hello in one word."}},
	}
}

// ─── Constructor ─────────────────────────────────────────────────────────────

func Test_GivenHost_WhenNew_ThenClientCanChat(t *testing.T) {
	// Given:
	server := httptest.NewServer(nonStreamingHandler("Hi"))
	defer server.Close()

	// When:
	client := New(server.URL)
	resp, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "Hi", resp.Content)
}

// ─── Non-streaming ───────────────────────────────────────────────────────────

func Test_GivenValidRequest_WhenChatNonStreaming_ThenReturnsContent(t *testing.T) {
	// Given:
	server := httptest.NewServer(nonStreamingHandler("Hello!"))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)
	req := baseRequest()

	// When:
	resp, err := client.Chat(context.Background(), req)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "Hello!", resp.Content)
}

func Test_GivenValidRequest_WhenChatNonStreaming_ThenPostsToApiChatPath(t *testing.T) {
	// Given:
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		nonStreamingHandler("Hello!")(w, r)
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "/api/chat", capturedPath)
}

func Test_GivenValidRequest_WhenChatNonStreaming_ThenRequestBodyContainsModel(t *testing.T) {
	// Given:
	var body ollamaRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		nonStreamingHandler("ok")(w, r)
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "llama3", body.Model)
}

func Test_GivenValidRequest_WhenChatNonStreaming_ThenRequestBodyContainsMessages(t *testing.T) {
	// Given:
	var body ollamaRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		nonStreamingHandler("ok")(w, r)
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.NoError(t, err)
	require.Len(t, body.Messages, 1)
	assert.Equal(t, "user", body.Messages[0].Role)
	assert.Equal(t, "Say hello in one word.", body.Messages[0].Content)
}

func Test_GivenValidRequest_WhenChatNonStreaming_ThenSetsContentTypeJSON(t *testing.T) {
	// Given:
	var capturedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContentType = r.Header.Get("Content-Type")
		nonStreamingHandler("ok")(w, r)
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "application/json", capturedContentType)
}

// ─── Streaming ───────────────────────────────────────────────────────────────

func Test_GivenStreamingRequest_WhenChat_ThenAccumulatesAllTokensInContent(t *testing.T) {
	// Given:
	server := httptest.NewServer(streamingHandler([]string{"Hel", "lo!"}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)
	req := baseRequest()
	req.Stream = true

	// When:
	resp, err := client.Chat(context.Background(), req)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "Hello!", resp.Content)
}

func Test_GivenStreamingRequest_WhenChat_ThenWritesEachTokenToWriter(t *testing.T) {
	// Given:
	server := httptest.NewServer(streamingHandler([]string{"Hel", "lo!"}))
	defer server.Close()
	var buf bytes.Buffer
	client := newWithWriter(server.URL, &buf)
	req := baseRequest()
	req.Stream = true

	// When:
	_, err := client.Chat(context.Background(), req)

	// Then:
	require.NoError(t, err)
	assert.Equal(t, "Hello!", buf.String())
}

// ─── Error handling ───────────────────────────────────────────────────────────

func Test_GivenConnectionRefused_WhenChat_ThenReturnsReachabilityError(t *testing.T) {
	// Given:
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	host := server.URL
	server.Close()
	client := newWithWriter(host, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not reach Ollama at")
}

func Test_GivenConnectionRefused_WhenChat_ThenErrorMessageContainsHost(t *testing.T) {
	// Given:
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	host := server.URL
	server.Close()
	client := newWithWriter(host, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), host)
}

func Test_GivenModelNotFound_WhenChat_ThenReturnsModelNotFoundError(t *testing.T) {
	// Given:
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)

	// When:
	_, err := client.Chat(context.Background(), baseRequest())

	// Then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in Ollama")
	assert.Contains(t, err.Error(), "ollama pull")
}

func Test_GivenModelNotFound_WhenChat_ThenErrorMessageContainsModelName(t *testing.T) {
	// Given:
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)
	req := baseRequest()

	// When:
	_, err := client.Chat(context.Background(), req)

	// Then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"llama3"`)
}

func Test_GivenContextTimeout_WhenChat_ThenReturnsTimeoutError(t *testing.T) {
	// Given:
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	defer server.Close()
	client := newWithWriter(server.URL, io.Discard)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// When:
	_, err := client.Chat(ctx, baseRequest())

	// Then:
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out after 120s")
}
