package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// Client implements domainllm.Provider against the OpenRouter API. Every
// successful call records its token usage and cost via recorder.
type Client struct {
	baseURL    string
	apiKey     string
	apiKeyMu   sync.RWMutex
	httpClient *http.Client
	recorder   domainllm.UsageRecorder
}

// NewClient creates a Client against baseURL, authenticating with apiKey
// and recording usage via recorder. An empty baseURL defaults to the real
// OpenRouter API.
func NewClient(baseURL, apiKey string, recorder domainllm.UsageRecorder) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		recorder:   recorder,
	}
}

// SetAPIKey updates the key used by subsequent Chat, ChatStream and
// Embeddings calls. Safe to call concurrently with those methods — e.g.
// from a Settings save while a study session is streaming a response. See
// specs/phases/phase-01-desktop-mvp/08-settings.md.
func (c *Client) SetAPIKey(key string) {
	c.apiKeyMu.Lock()
	defer c.apiKeyMu.Unlock()
	c.apiKey = key
}

func (c *Client) key() string {
	c.apiKeyMu.RLock()
	defer c.apiKeyMu.RUnlock()
	return c.apiKey
}

type openRouterUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage openRouterUsage `json:"usage"`
}

type chatCompletionChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *openRouterUsage `json:"usage"`
}

type embeddingsRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage openRouterUsage `json:"usage"`
}

// Chat sends a single chat completion request. If the account has run out
// of credits, it retries once against domainllm.FreeFallbackModel.
func (c *Client) Chat(ctx context.Context, req domainllm.ChatRequest) (domainllm.ChatResponse, error) {
	if c.key() == "" {
		return domainllm.ChatResponse{}, domainllm.ErrAPIKeyMissing
	}

	model := domainllm.ModelFor(req.Task)
	completion, err := c.chatCompletion(ctx, model, req.Messages, false)
	usedFallback := false
	if errors.Is(err, domainllm.ErrInsufficientCredits) {
		model = domainllm.FreeFallbackModel
		usedFallback = true
		completion, err = c.chatCompletion(ctx, model, req.Messages, false)
	}
	if err != nil {
		return domainllm.ChatResponse{}, err
	}

	if err := c.record(ctx, req.SessionID, model, completion.Usage); err != nil {
		return domainllm.ChatResponse{}, err
	}

	var content string
	if len(completion.Choices) > 0 {
		content = completion.Choices[0].Message.Content
	}
	return domainllm.ChatResponse{
		Content:          content,
		Model:            model,
		Usage:            usageFrom(completion.Usage),
		UsedFreeFallback: usedFallback,
	}, nil
}

func (c *Client) chatCompletion(ctx context.Context, model string, messages []domainllm.Message, stream bool) (chatCompletionResponse, error) {
	resp, err := c.post(ctx, "/api/v1/chat/completions", chatCompletionRequest{
		Model:    model,
		Messages: toChatMessages(messages),
		Stream:   stream,
	})
	if err != nil {
		return chatCompletionResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return chatCompletionResponse{}, fmt.Errorf("openrouter: decoding chat completion response: %w", err)
	}
	return decoded, nil
}

// ChatStream sends a streaming chat completion request, calling handler
// once per non-empty content delta until the stream ends. If the account
// has run out of credits, it retries once against
// domainllm.FreeFallbackModel before delivering any chunk to handler.
func (c *Client) ChatStream(ctx context.Context, req domainllm.ChatRequest, handler func(chunk string) error) (domainllm.StreamResponse, error) {
	if c.key() == "" {
		return domainllm.StreamResponse{}, domainllm.ErrAPIKeyMissing
	}

	model := domainllm.ModelFor(req.Task)
	result, err := c.streamChatCompletion(ctx, model, req.Messages, handler)
	usedFallback := false
	if errors.Is(err, domainllm.ErrInsufficientCredits) {
		model = domainllm.FreeFallbackModel
		usedFallback = true
		result, err = c.streamChatCompletion(ctx, model, req.Messages, handler)
	}
	if err != nil {
		return domainllm.StreamResponse{}, err
	}

	// Record usage under the same resolved model reported on StreamResponse
	// (the concrete model behind an auto-router alias like
	// domainllm.FreeFallbackModel), including empty when the stream's
	// metadata wasn't trustworthy — recording under the requested alias
	// instead would attribute cost/usage to an unconfirmed model.
	if err := c.record(ctx, req.SessionID, result.model, result.usage); err != nil {
		return domainllm.StreamResponse{}, err
	}

	return domainllm.StreamResponse{
		Model:            result.model,
		Usage:            usageFrom(result.usage),
		UsedFreeFallback: usedFallback,
	}, nil
}

// streamResult is streamChatCompletion's return: the final usage frame plus
// the model resolved from the stream's own "model" field, which may differ
// from the model requested (e.g. domainllm.FreeFallbackModel is an
// auto-router alias, not a concrete model).
type streamResult struct {
	// model is the exact ID every non-empty per-chunk "model" value agreed
	// on, or "" if no chunk reported one or two chunks disagreed — never
	// invented.
	model string
	usage openRouterUsage
}

func (c *Client) streamChatCompletion(ctx context.Context, model string, messages []domainllm.Message, handler func(chunk string) error) (streamResult, error) {
	resp, err := c.post(ctx, "/api/v1/chat/completions", chatCompletionRequest{
		Model:    model,
		Messages: toChatMessages(messages),
		Stream:   true,
	})
	if err != nil {
		return streamResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var lastUsage openRouterUsage
	var resolvedModel string
	modelConflict := false
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return streamResult{}, fmt.Errorf("openrouter: decoding stream chunk: %w", err)
		}
		if chunk.Usage != nil {
			lastUsage = *chunk.Usage
		}
		if chunk.Model != "" {
			switch resolvedModel {
			case "":
				resolvedModel = chunk.Model
			case chunk.Model:
				// same model reported again, no conflict.
			default:
				modelConflict = true
			}
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		if err := handler(chunk.Choices[0].Delta.Content); err != nil {
			return streamResult{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return streamResult{}, fmt.Errorf("openrouter: reading stream: %w", err)
	}
	if modelConflict {
		resolvedModel = ""
	}

	return streamResult{model: resolvedModel, usage: lastUsage}, nil
}

// Embeddings requests a vector embedding for a single input text. There is
// no free-model fallback for embeddings: insufficient credits fails
// directly with domainllm.ErrInsufficientCredits.
func (c *Client) Embeddings(ctx context.Context, req domainllm.EmbeddingRequest) (domainllm.EmbeddingResponse, error) {
	if c.key() == "" {
		return domainllm.EmbeddingResponse{}, domainllm.ErrAPIKeyMissing
	}

	resp, err := c.post(ctx, "/api/v1/embeddings", embeddingsRequest{
		Model: domainllm.EmbeddingModel,
		Input: req.Input,
	})
	if err != nil {
		return domainllm.EmbeddingResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return domainllm.EmbeddingResponse{}, fmt.Errorf("openrouter: decoding embeddings response: %w", err)
	}

	if err := c.record(ctx, req.SessionID, domainllm.EmbeddingModel, decoded.Usage); err != nil {
		return domainllm.EmbeddingResponse{}, err
	}

	var embedding []float64
	if len(decoded.Data) > 0 {
		embedding = decoded.Data[0].Embedding
	}
	return domainllm.EmbeddingResponse{
		Embedding: embedding,
		Model:     domainllm.EmbeddingModel,
		Usage:     usageFrom(decoded.Usage),
	}, nil
}

// post sends a JSON-encoded body to path and maps well-known error status
// codes to domain sentinels. On success, the caller owns closing the
// response body.
func (c *Client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openrouter: encoding request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("openrouter: building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.key())
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter: calling %s: %w", path, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusUnauthorized:
		_ = resp.Body.Close()
		return nil, domainllm.ErrAPIKeyInvalid
	case http.StatusPaymentRequired:
		_ = resp.Body.Close()
		return nil, domainllm.ErrInsufficientCredits
	default:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("openrouter: %s returned status %d", path, resp.StatusCode)
	}
}

// get sends a GET request to path, without requiring an API key (used by
// ListModels, which OpenRouter serves publicly and which the catalog
// warm-up may call before onboarding has set one). On success, the caller
// owns closing the response body.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("openrouter: building request: %w", err)
	}
	if key := c.key(); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter: calling %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("openrouter: %s returned status %d", path, resp.StatusCode)
	}
	return resp, nil
}

func (c *Client) record(ctx context.Context, sessionID, model string, usage openRouterUsage) error {
	entry := domainllm.UsageEntry{
		ID:           uuid.NewString(),
		SessionID:    sessionID,
		Model:        model,
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		Cost:         usage.Cost,
		CreatedAt:    time.Now(),
	}
	if err := c.recorder.Record(ctx, entry); err != nil {
		return fmt.Errorf("openrouter: recording usage: %w", err)
	}
	return nil
}

func usageFrom(u openRouterUsage) domainllm.Usage {
	return domainllm.Usage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		Cost:         u.Cost,
	}
}

func toChatMessages(messages []domainllm.Message) []chatMessage {
	out := make([]chatMessage, len(messages))
	for i, m := range messages {
		out[i] = chatMessage{Role: m.Role, Content: m.Content}
	}
	return out
}
