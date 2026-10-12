// Package llm holds the Provider port backed by OpenRouter — chat,
// streaming and embeddings — plus the model routing and usage tracking
// rules that sit in front of it. See
// specs/phases/phase-01-desktop-mvp/05-llm-service.md.
package llm

import (
	"context"
	"errors"
)

// Sentinel errors returned by Provider implementations.
var (
	// ErrAPIKeyMissing is returned when no OpenRouter API key is configured.
	ErrAPIKeyMissing = errors.New("openrouter api key is missing")
	// ErrAPIKeyInvalid is returned when OpenRouter rejects the API key.
	ErrAPIKeyInvalid = errors.New("openrouter api key is invalid or unauthorized")
	// ErrInsufficientCredits is returned when the OpenRouter account has run
	// out of credits and no free-model fallback succeeded either.
	ErrInsufficientCredits = errors.New("openrouter account has insufficient credits")
)

// Message is a single turn in a chat conversation.
type Message struct {
	Role    string // system | user | assistant
	Content string
}

// Usage reports the tokens consumed and cost billed for a single LLM call.
type Usage struct {
	InputTokens  int
	OutputTokens int
	Cost         float64
}

// ChatRequest is a request to Provider.Chat or Provider.ChatStream.
type ChatRequest struct {
	SessionID string
	Task      TaskType
	Messages  []Message
}

// ChatResponse is the result of a successful Provider.Chat call.
type ChatResponse struct {
	Content string
	Model   string
	Usage   Usage
	// UsedFreeFallback is true when the request initially failed with
	// insufficient credits and was retried against FreeFallbackModel.
	UsedFreeFallback bool
}

// EmbeddingRequest is a request to Provider.Embeddings.
type EmbeddingRequest struct {
	SessionID string
	Input     string
}

// EmbeddingResponse is the result of a successful Provider.Embeddings
// call.
type EmbeddingResponse struct {
	Embedding []float64
	Model     string
	Usage     Usage
}

// EmbeddingBatchRequest is a request to Provider.EmbeddingsBatch — several
// inputs embedded in a single round trip. A separate request type from
// EmbeddingRequest (rather than widening Input to []string) because
// EmbeddingRequest's only other caller (query embedding, in
// application/knowledge) is single-text by nature and has no reason to
// become batch-aware.
type EmbeddingBatchRequest struct {
	SessionID string
	Inputs    []string
}

// EmbeddingResult is one EmbeddingBatchResponse entry, in the same order
// as its request's Inputs.
type EmbeddingResult struct {
	Embedding []float64
}

// EmbeddingBatchResponse is the result of a successful
// Provider.EmbeddingsBatch call. Usage is the batch's total — OpenRouter
// reports one usage block per request, not per input.
type EmbeddingBatchResponse struct {
	Embeddings []EmbeddingResult
	Model      string
	Usage      Usage
}

// StreamResponse is the result of a successful Provider.ChatStream call:
// the actual model that produced the response (which may differ from the
// requested router alias, e.g. FreeFallbackModel) and its usage, reported
// only once the stream completes.
type StreamResponse struct {
	Model string
	Usage Usage
	// UsedFreeFallback is true when the request initially failed with
	// insufficient credits and was retried against FreeFallbackModel.
	UsedFreeFallback bool
}

// Provider is the port every LLM backend adapter implements.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, handler func(chunk string) error) (StreamResponse, error)
	Embeddings(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
	EmbeddingsBatch(ctx context.Context, req EmbeddingBatchRequest) (EmbeddingBatchResponse, error)
}
