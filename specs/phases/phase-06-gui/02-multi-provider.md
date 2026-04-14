# Spec: Multi-Provider Support (OpenAI, Claude, Gemini)

## Goal

Extend the `LLMProvider` interface to support cloud providers so users can choose between local (Ollama) and cloud AI. Each provider is plug-and-play thanks to the interface defined in Phase 1.

## User Story

> As a developer, I want to use OpenAI GPT-4o for interview sessions while keeping Ollama for daily study, so I can balance cost and quality.

## Acceptance Criteria

- [ ] OpenAI provider implements `LLMProvider` (chat + embeddings)
- [ ] Anthropic (Claude) provider implements `LLMProvider` (chat only initially)
- [ ] Google Gemini provider implements `LLMProvider` (chat only initially)
- [ ] API keys are stored in config, never hardcoded
- [ ] `athena config set openai.api_key <key>` stores the key securely (file permissions 600)
- [ ] `--provider openai --model gpt-4o` works on any command
- [ ] Missing API key produces a clear error: `OpenAI API key not set — run: athena config set openai.api_key <key>`

## CLI Usage

```bash
athena config set provider openai
athena config set openai.api_key sk-...
athena config set openai.model gpt-4o

athena study caching --provider openai
athena interview system-design --provider claude --model claude-opus-4-6
```

## Config Extension

```yaml
provider: ollama
model: llama3

ollama:
  host: http://localhost:11434

openai:
  api_key: sk-...
  model: gpt-4o

anthropic:
  api_key: sk-ant-...
  model: claude-opus-4-6

gemini:
  api_key: AIza...
  model: gemini-2.0-flash
```

## Directory Structure

```
internal/
└── llm/
    ├── provider.go
    ├── factory.go
    ├── ollama/
    │   └── ollama.go
    ├── openai/
    │   └── openai.go
    ├── anthropic/
    │   └── anthropic.go
    └── gemini/
        └── gemini.go
```

## Implementation Notes

- Each provider lives in its own sub-package and only knows about the `LLMProvider` interface
- Use official SDKs where available:
  - OpenAI: `github.com/openai/openai-go`
  - Anthropic: `github.com/anthropics/anthropic-sdk-go`
  - Gemini: `google.golang.org/genai`
- Config file permissions: `os.Chmod(configPath, 0600)` after writing API keys
- Streaming support is required for all providers

## Done When

```bash
$ athena config set openai.api_key sk-test
$ athena study caching --provider openai --model gpt-4o
# → session runs using OpenAI API
```
