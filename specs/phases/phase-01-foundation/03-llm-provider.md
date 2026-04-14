# Spec: LLM Provider Abstraction + Ollama

## Goal

Define the `LLMProvider` interface and implement it for Ollama. All AI features depend on this layer — it must be in place before any command that talks to a model.

## User Story

> As a developer, I want the tool to talk to my local Ollama instance so I can use it offline without API costs.

## Acceptance Criteria

- [ ] `LLMProvider` interface is defined in `internal/platform/llm`
- [ ] Ollama provider implements the interface
- [ ] `athena config set provider ollama` selects it
- [ ] A call to `Chat()` reaches Ollama and returns a response
- [ ] Connection errors produce a clear message: `"could not reach Ollama at <host> — is it running?"`
- [ ] Provider can be overridden per-command with `--provider` flag

## Interface

```go
// internal/platform/llm/provider.go

type ChatMessage struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

type ChatRequest struct {
    Model    string
    Messages []ChatMessage
    Stream   bool
}

type ChatResponse struct {
    Content string
}

type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
```

## Directory Structure

```
internal/
└── platform/
    └── llm/
        ├── provider.go          # LLMProvider interface
        ├── factory.go           # NewProvider(name, host string) LLMProvider
        └── ollama/
            ├── ollama.go        # HTTP client implementation
            └── ollama_test.go
```

`llm` is a foundational package inside `internal/platform/`. It must **not** import `internal/platform/config/` — packages at the same level cannot import each other. The `cmd/` layer reads config and passes primitives to the factory.

## Factory

```go
// internal/platform/llm/factory.go

// NewProvider receives primitive values from cmd/ — it does not import config.
func NewProvider(name, host string) (LLMProvider, error) {
    switch name {
    case "ollama":
        return ollama.New(host), nil
    default:
        return nil, fmt.Errorf("unknown provider: %s", name)
    }
}
```

The wiring in `cmd/athena/` is responsible for reading config and calling the factory:

```go
// cmd/athena/root.go (or per-command file)
cfg, _ := config.Load()
provider, _ := llm.NewProvider(cfg.Provider, cfg.Ollama.Host)
```

## Ollama Implementation Notes

- Use the Ollama `/api/chat` endpoint
- Support streaming output (print tokens as they arrive)
- Default timeout: 120 seconds
- The `ollama.New(host)` constructor accepts the base URL from config

## Error Handling

| Situation | Message |
|---|---|
| Connection refused | `could not reach Ollama at <host> — is it running?` |
| Model not found | `model "<name>" not found in Ollama — run: ollama pull <name>` |
| Context timeout | `request timed out after 120s` |

## Done When

An internal smoke test (or manual test) shows:

```go
provider := ollama.New("http://localhost:11434")
resp, err := provider.Chat(ctx, ChatRequest{
    Model: "llama3",
    Messages: []ChatMessage{
        {Role: "user", Content: "Say hello in one word."},
    },
})
// resp.Content == "Hello!"
```
