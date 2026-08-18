# Phase 2.5 — RAG Integration

## Goal

Every study turn first checks local knowledge. The LLM receives retrieved chunks as context, and in `strict-notes` with no matches it is not called at all.

## Flow

```text
User question
    ↓
Source mode = web?  → plain LLM call, no retrieval
    ↓ no
Vector store empty? → plain LLM call, no embedding spent
    ↓ no
Embed the question → vector search → top-K chunks above minScore
    ↓
No chunks and mode = strict-notes → "No local knowledge found", NO LLM call
    ↓
Otherwise → LLM call with the chunks injected as context
```

## Source Modes

| Mode | Chunks found | Behavior |
|---|---|---|
| `web` | — | No embedding, no search, plain LLM call (today's behavior) |
| `notes` | none | Plain LLM call |
| `notes` | yes | LLM call + context block, instructed to prefer it |
| `strict-notes` | none | **No LLM call.** Fixed response: "No local knowledge found for this question." |
| `strict-notes` | yes | LLM call + context, instructed to answer *exclusively* from it and to say so if it is insufficient |

Interpretive call to record: "answer using local knowledge only" means *the LLM answers constrained to the retrieved context*, not that raw chunk text is returned as the assistant turn — a Socratic tutor that pastes note fragments is unusable. The only explicit no-LLM criterion is `strict-notes` with no matches. `Sufficient` selects the instruction wording; it does not skip the model.

**The mode is transient and passed per call** — `SendStudyMessage(sessionID, topic, content, sourceMode)`. `topic` is already re-sent every turn, so this needs no migration and no persistence semantics, and the user can flip modes mid-session.

## Retrieval

```go
const (
    SourceModeNotes       = "notes"
    SourceModeStrictNotes = "strict-notes"
    SourceModeWeb         = "web"
)

type RetrievalResult struct {
    Chunks     []ScoredChunk
    Sufficient bool
    Context    string   // rendered block, already capped
    Sources    []Source // {FilePath, Heading, Concept, Score}
}

func (s *Service) Retrieve(ctx context.Context, query, mode string) (RetrievalResult, error)
```

**Thresholds are constructor-injected, not hardcoded** — the requirement is a configurable minimum similarity score. `NewService(..., minScore, sufficiencyScore float64)` with defaults exported from the domain: `DefaultMinSimilarity = 0.35`, `DefaultSufficiency = 0.55`, wired in `main.go`. These are calibrated for `text-embedding-3-small`, whose question↔passage cosines typically land in 0.35–0.65; a 0.75-style threshold would silently disable retrieval. Surfacing them in Settings is a follow-up, not this phase.

**Context-window cap:** `maxContextChars = 8000` (~2k tokens). When over budget, **drop whole chunks lowest-score-first** — never truncate mid-chunk, so the cited sources always match what the model actually saw.

## Wiring into the study session

`study.Service` gains a **port**, not a concrete dependency: `knowledge.Retriever` is defined in `internal/domain/knowledge` and implemented by `application/knowledge.Service` (`RetrievalResult` moves to the domain). Doing the retrieval in the desktop binding instead would put orchestration in an adapter, which ADR-001 forbids.

The context is injected as a **second `system` message**, immediately after the existing one, built by `buildKnowledgeContext(result)` in a new `internal/application/study/prompt_context.go`. `buildSystemPrompt` and its seven tests stay untouched, and the context block becomes independently unit-testable.

> `study.NewService` grows to six parameters, which touches every study test file (each constructs five mocks). Mechanical, but budget for it. Any later refactor that "cleans up" by merging the two system messages re-breaks `prompt_test.go`.

## Source citation

A new `study:sources` event is emitted **before** the stream starts, payload `[{filePath, heading, concept, score}]`, rendered as a collapsible "Sources" strip under the assistant bubble. Deterministic and testable, unlike depending on the model to format inline citation markers.

Sources are transient: `messages` has no sources column, so they do not reappear on resume. A `message_sources` table is Phase 3 scope.

## Tasks

- [ ] `internal/domain/knowledge/retrieval.go` — `RetrievalResult`, `Source`, `Retriever` port, source-mode constants, `DefaultMinSimilarity` / `DefaultSufficiency`
- [ ] `internal/application/knowledge/retrieval.go` — `Retrieve`, threshold handling, context rendering with the char cap
- [ ] `internal/application/study/prompt_context.go` — `buildKnowledgeContext`
- [ ] `internal/application/study/send_message.go` — accept `sourceMode`, call the retriever, inject the second system message, short-circuit the `strict-notes`-with-no-matches case
- [ ] `internal/interfaces/desktop/study.go` — `SendStudyMessage` gains `sourceMode`; emit `study:sources`
- [ ] `main.go` — wire the retriever into `study.NewService` with the default thresholds
- [ ] `frontend/src/lib/study.ts` — fourth `sendStudyMessage` argument + `onStudySources`
- [ ] `frontend/src/screens/StudyChatScreen.tsx` — source-mode `Select` next to the composer (default `notes`), collapsible Sources strip

## Acceptance Criteria

- With `strict-notes` and no matching chunks, the app responds "No local knowledge found for this question" and makes **no LLM call** (no new row in `usage`)
- With `notes` mode and matching chunks, the response references the imported content and the Sources strip lists the files
- With `web` mode, no embedding is requested and no retrieval happens
- With an empty vector store, no embedding is requested regardless of mode
- The injected context never exceeds `maxContextChars`, and over-budget chunks are dropped whole, lowest score first
- Cited sources are exactly the chunks that survived the cap
- Thresholds are injected, and a raised `minScore` demonstrably filters low-similarity chunks out
- The existing `buildSystemPrompt` tests still pass unchanged
