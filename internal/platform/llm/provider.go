package llm

import "context"

// ChatMessage is a single message in a conversation turn.
type ChatMessage struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

// ChatRequest describes a request to the LLM.
type ChatRequest struct {
	Model    string
	Messages []ChatMessage
	Stream   bool
}

// ChatResponse holds the model's reply.
type ChatResponse struct {
	Content string
}

// LLMProvider is the interface all AI backends must satisfy.
type LLMProvider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
