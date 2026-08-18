# Phase 2.5 — RAG Integration

## Goal

Every study turn first checks local knowledge. The LLM receives retrieved chunks as context, and in `strict-notes` with no matches it is not called at all.

## Flow

```text
User question
    ↓
Source mode = web?  → plain LLM call, no retrieval
    ↓ no
Vector store empty? → no chunks, and no embedding spent   ─┐
    ↓ no                                                    │
Embed the question → vector search → chunks above minScore ─┤
    ↓                                                       │
    ├── no chunks ←──────────────────────────────────────────┘
    │      ↓
    │   mode = strict-notes → NO_LOCAL_KNOWLEDGE, NO LLM call
    │      ↓ mode = notes
    │   plain LLM call
    ↓
chunks found → LLM call with the chunks injected as context
```

An empty vector store is just the cheapest way of finding no chunks — it **must not** short-circuit past the mode check. Testing it first would make `strict-notes` on a fresh install call the LLM, contradicting the table and the acceptance criteria below.

## Source Modes

| Mode | Chunks found | Behavior |
|---|---|---|
| `web` | — | No embedding, no search, plain LLM call (today's behavior) |
| `notes` | none | Plain LLM call |
| `notes` | yes | LLM call + context block, instructed to prefer it |
| `strict-notes` | none | **No LLM call.** Fixed response `NoLocalKnowledgeMessage` (see below) |
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
    Sources    []Source // {ChunkID, ItemID, FilePath, Heading, Concept, Score, Excerpt}
}

func (s *Service) Retrieve(ctx context.Context, query, mode string) (RetrievalResult, error)
```

The fixed no-knowledge reply is a single exported constant, so the flow, the implementation, and the tests cannot drift apart:

```go
const NoLocalKnowledgeMessage = "No local knowledge found for this question."
```

**`topK` is `DefaultTopK = 8`**, exported from the domain alongside the thresholds and passed to `VectorStore.Search`. It is deliberately a little larger than what the context budget will fit: the cap below drops the weakest chunks afterwards, so retrieval over-fetches slightly and then trims by score rather than truncating the search itself.

**Thresholds are constructor-injected, not hardcoded** — the requirement is a configurable minimum similarity score. `NewService(..., minScore, sufficiencyScore float64)` with defaults exported from the domain: `DefaultMinSimilarity = 0.35`, `DefaultSufficiency = 0.55`, wired in `main.go`. These are calibrated for `text-embedding-3-small`, whose question↔passage cosines typically land in 0.35–0.65; a 0.75-style threshold would silently disable retrieval. Surfacing them in Settings is a follow-up, not this phase.

**Context-window cap:** `maxContextChars = 8000` (~2k tokens). When over budget, **drop whole chunks lowest-score-first** — never truncate mid-chunk, so the cited sources always match what the model actually saw.

## Wiring into the study session

`study.Service` gains a **port**, not a concrete dependency: `knowledge.Retriever` is defined in `internal/domain/knowledge` and implemented by `application/knowledge.Service` (`RetrievalResult` moves to the domain). Doing the retrieval in the desktop binding instead would put orchestration in an adapter, which ADR-001 forbids.

The context is injected as a **second `system` message**, immediately after the existing one, built by `buildKnowledgeContext(result)` in a new `internal/application/study/prompt_context.go`. `buildSystemPrompt` and its seven tests stay untouched, and the context block becomes independently unit-testable.

> `study.NewService` grows to six parameters, which touches every study test file (each constructs five mocks). Mechanical, but budget for it. Any later refactor that "cleans up" by merging the two system messages re-breaks `prompt_test.go`.

## Source citation

A new `study:sources` event is emitted **before** the stream starts, payload `[{filePath, heading, concept, score}]`, rendered as a collapsible "Sources" strip under the assistant bubble. Deterministic and testable, unlike depending on the model to format inline citation markers.

In this spec the event is transient. Spec 2.9 adds `message_sources` and atomically
persists the exact post-cap source list with the completed assistant message, so
sources reappear on resume. Persistent response provenance is therefore Phase 2
scope, not Phase 3.

## Tasks

- [ ] `internal/domain/knowledge/retrieval.go` — `RetrievalResult`, `Source`, `Retriever` port, source-mode constants, `NoLocalKnowledgeMessage`, `DefaultTopK`, `DefaultMinSimilarity` / `DefaultSufficiency`
- [ ] `internal/application/knowledge/retrieval.go` — `Retrieve`, threshold handling, context rendering with the char cap
- [ ] `internal/application/study/prompt_context.go` — `buildKnowledgeContext`
- [ ] `internal/application/study/send_message.go` — accept `sourceMode`, call the retriever, inject the second system message, short-circuit the `strict-notes`-with-no-matches case
- [ ] `internal/interfaces/desktop/study.go` — `SendStudyMessage` gains `sourceMode`; emit `study:sources`
- [ ] `main.go` — wire the retriever into `study.NewService` with the default thresholds
- [ ] `frontend/src/lib/study.ts` — fourth `sendStudyMessage` argument + `onStudySources`
- [ ] `frontend/src/screens/StudyChatScreen.tsx` — source-mode `Select` next to the composer (default `notes`), collapsible Sources strip

## Acceptance Criteria

- With `strict-notes` and no matching chunks, the app responds with `NoLocalKnowledgeMessage` and makes **no LLM call** (no new row in `usage`)
- With `strict-notes` and an **empty** vector store, the same holds: no embedding is requested *and* no LLM call is made — an empty store must not be mistaken for permission to fall back
- With `notes` mode and matching chunks, the response references the imported content and the Sources strip lists the files
- With `web` mode, no embedding is requested and no retrieval happens
- With an empty vector store, no embedding is requested regardless of mode; in `notes` and `web` this falls through to a plain LLM call, in `strict-notes` it does not
- The injected context never exceeds `maxContextChars`, and over-budget chunks are dropped whole, lowest score first
- Cited sources are exactly the chunks that survived the cap
- After 2.9, those exact sources reappear in the same order after resuming the session
- Thresholds are injected, and a raised `minScore` demonstrably filters low-similarity chunks out
- The existing `buildSystemPrompt` tests still pass unchanged
