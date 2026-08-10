# Phase 1.5 — LLM Service (OpenRouter)

## Goal

Single, testable `LLMProvider` abstraction backed by OpenRouter, supporting streaming, embeddings, model routing, and cost tracking.

## Interface

```go
type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest, handler func(chunk string) error) error
    Embeddings(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}
```

## Tasks

- [ ] `internal/infrastructure/openrouter/` — HTTP client implementing `LLMProvider`
- [ ] Streaming via SSE (OpenRouter Server-Sent Events)
- [ ] Model router: maps task type → model tier (`cheap | medium | premium`)
- [ ] Budget tracker: records tokens and cost per session
- [ ] OpenRouter API key config: `~/.athena/config.yaml`

## Model Tiers (initial)

| Tier | Use case |
|---|---|
| cheap | Onboarding, knowledge extraction, embeddings |
| medium | Study mode, challenge feedback |
| premium | Interview evaluation, complex reasoning |

## Acceptance Criteria

- `LLMProvider.Chat` returns a response for a simple prompt
- `LLMProvider.ChatStream` calls the handler once per chunk until done
- `LLMProvider.Embeddings` returns a float slice for a given input text
- Token count and estimated cost are recorded in the `usage` table after each call
- Missing or invalid API key returns a descriptive error (not a panic)
