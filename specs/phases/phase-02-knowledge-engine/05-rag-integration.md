# Phase 2.5 — RAG Integration

## Goal

Every LLM call first checks local knowledge. The LLM is called only when local knowledge is insufficient.

## Flow

```text
User question
    ↓
Embed the question
    ↓
Vector search → top-K relevant chunks
    ↓
Sufficient local knowledge?
    YES → answer using local knowledge only
    NO  → call OpenRouter with chunks as context
```

## Source Modes (selectable in UI)

| Mode | Behavior |
|---|---|
| `notes` | Prefer local notes; fall back to LLM if insufficient |
| `strict-notes` | Only use local notes; no LLM fallback |
| `web` | Always use LLM (no local retrieval) |

## Tasks

- [ ] `internal/application/knowledge/retrieval.go` — retrieval use case
- [ ] Sufficiency threshold: configurable minimum similarity score
- [ ] Chunks injected into system prompt as structured context
- [ ] Source mode selector in the study session UI
- [ ] Sources cited in the response (which files/chunks were used)

## Acceptance Criteria

- With `strict-notes` and no matching chunks, the app responds "no local knowledge found" instead of calling the LLM
- With `notes` mode and matching chunks, the response references the imported content
- The injected context does not exceed the model's context window limit
