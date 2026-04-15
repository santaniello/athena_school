package llm

import (
	"context"
	"fmt"

	"github.com/fsantaniello/athena_school/internal/platform/llm/ollama"
)

// NewProvider constructs an LLMProvider from primitive values.
// The cmd/ layer is responsible for reading config and passing these values.
func NewProvider(name, host string) (LLMProvider, error) {
	switch name {
	case "ollama":
		return &ollamaAdapter{client: ollama.New(host)}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

// ollamaAdapter bridges between the llm.LLMProvider interface and ollama.Client,
// converting between llm.ChatRequest/ChatResponse and ollama.Request/Response.
type ollamaAdapter struct {
	client *ollama.Client
}

func (a *ollamaAdapter) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	msgs := make([]ollama.Message, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollama.Message{Role: m.Role, Content: m.Content}
	}
	resp, err := a.client.Chat(ctx, ollama.Request{
		Model:    req.Model,
		Messages: msgs,
		Stream:   req.Stream,
	})
	if err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{Content: resp.Content}, nil
}
