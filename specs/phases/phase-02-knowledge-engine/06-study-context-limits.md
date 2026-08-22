# Phase 2.6 — Study Context Limits

## Goal

Warn before a study session exhausts the active model's context window, then
prevent further sends before reliability collapses. The user can continue by
opening a new session in the same folder and on the same topic; Athena never
silently deletes, truncates, or summarizes the existing conversation.

This is separate from Phase 2.5's `maxContextChars = 8000`. That cap applies only
to the transient RAG data block rebuilt for one turn. This specification manages
the full model request, whose persisted conversation history grows every turn.

## User-visible states

```go
type ContextState string

const (
    ContextStateNormal  ContextState = "normal"
    ContextStateWarning ContextState = "warning"
    ContextStateBlocked ContextState = "blocked"
)
```

For a known positive model context length:

```text
used tokens < 80%  → normal
used tokens ≥ 80%  → warning
used tokens ≥ 95%  → blocked
```

Use integer comparisons rather than floating-point percentages. Equality enters
the higher state.

At `warning`, show a persistent, non-blocking informational alert above the
composer:

```text
This session is approaching the model's context limit.
Start a new session on the same topic to keep responses reliable.
```

At `blocked`, replace it with:

```text
This session has reached its context limit.
Start a new session on the same topic to continue.
```

The blocked state:

- disables the send button;
- prevents Enter from sending;
- disables the Phase 2.5 source-mode selector;
- is enforced in `study.Service` with exported
  `ErrSessionContextLimitReached`, so a forged desktop call cannot bypass it;
- does not prevent reading, navigating, or extracting knowledge from the
  session.

There is no automatic history truncation, deletion, or summarization. If a
provider rejects a request before Athena has observed enough usage to warn, the
existing error flow reports it normally.

## Continue in a new session

Both warning and blocked alerts include `Start new session`. The action:

1. creates a study session in the current session's folder;
2. reuses its topic;
3. navigates to the new chat;
4. requests the ordinary opening turn;
5. copies no messages, RAG sources, summaries, or source-mode selection.

The old session remains intact and resumable for reading.

## Measuring the context

### Streaming metadata

The current streaming port discards response metadata. Change it to return the
actual model and usage reported by the completed stream:

```go
type StreamResponse struct {
    Model            string
    Usage            Usage
    UsedFreeFallback bool
}

type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    ChatStream(
        ctx context.Context,
        req ChatRequest,
        handler func(chunk string) error,
    ) (StreamResponse, error)
    Embeddings(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}
```

The OpenRouter adapter parses the resolved model from the stream rather than
recording only the requested router alias. It returns the final usage frame and
records that same resolved model and usage through `UsageRecorder`. One
successful stream has one resolved model, including a successful free fallback.

After a successful opening or study response, the new occupancy is:

```text
Usage.InputTokens + Usage.OutputTokens
```

The output is included because the completed assistant response becomes part of
the next request's history. This is exact for the completed call and predictive
for the next one; the 80% warning and 95% block leave room for the next user
message, system/RAG context, and response.

If a successful stream lacks usable positive usage metadata, Athena falls back
to the conservative estimate below for the complete assembled request and
response and marks the measurement as estimated. Missing metadata never turns a
successful study response into a failure.

### Turns without a chat call

Messages can be persisted without a completed stream: most notably a
`strict-notes` miss, or a user message followed by a retrieval/provider error.
Those messages still become part of a later request.

For each such persisted message:

```text
estimated tokens = ceil(Unicode code points / 3) + 8
```

The same conservative formula applies regardless of the profile language;
English, Portuguese, code, and mixed-language messages may coexist. The eight
tokens cover per-message role and structural overhead.

Before retrieval or chat begins, persist the user message and its provisional
estimated increment atomically. If the turn later succeeds through the LLM, the
provider's real `InputTokens + OutputTokens` replaces the accumulated estimate.
If retrieval or chat fails, the user message remains—as Study Mode already
requires—and so does its estimated increment. A strict fixed assistant response
adds its own estimate.

Until the first real measurement exists, occupancy is the sum of these
estimates. Every later real measurement becomes the new base; only subsequently
persisted no-stream messages are estimated on top of it.

## Dynamic model catalog

Context length is runtime metadata, not a hardcoded map. Define a model-catalog
port and a cache service:

```go
type ModelInfo struct {
    ID            string
    ContextLength int
}

type ModelCatalog interface {
    ListModels(ctx context.Context) ([]ModelInfo, error)
}

type ModelContextResolver interface {
    ContextLength(ctx context.Context, modelID string) (int, error)
}
```

The OpenRouter adapter implements `ModelCatalog`. An application service indexes
the returned models by exact ID and implements `ModelContextResolver`.

The composition root starts one asynchronous catalog load per application
launch. Results stay in memory for that process. A completed stream whose
resolved model is absent triggers one on-demand refresh; no study message makes
an unconditional catalog request.

Catalog failure never blocks chat. If startup and on-demand refresh both fail,
preserve the session's last known context state and emit at most once per open
session:

```text
Unable to determine this session's context limit.
```

The warning is transient technical feedback. Normal context evaluation resumes
automatically after a later successful catalog refresh.

This dynamic boundary is also the foundation for a future user-selectable model
feature, but model selection itself is outside this phase.

## Persistence and atomicity

Persist enough state to restore the alert or block without requiring another LLM
call:

```go
type ContextUsage struct {
    State         ContextState
    Model         string
    UsedTokens    int
    ContextLength int
    Estimated     bool
}

type Session struct {
    // existing fields...
    Context ContextUsage
}
```

Add backward-compatible `sessions` columns with defaults representing a normal,
unmeasured session:

```sql
context_state          TEXT    NOT NULL DEFAULT 'normal',
context_model          TEXT    NOT NULL DEFAULT '',
context_used_tokens    INTEGER NOT NULL DEFAULT 0,
context_length         INTEGER NOT NULL DEFAULT 0,
context_estimated      INTEGER NOT NULL DEFAULT 0
```

Repository reads validate the enum and non-negative numeric fields. Unknown
state, negative counts, or an estimated value outside `0|1` is a decode error,
not silently normalized data.

The study application defines a transaction port implemented by the existing
SQLite transactor. The following writes are atomic:

- user message plus its provisional estimated increment;
- streamed assistant message plus the real/replacement context measurement;
- strict fixed assistant message plus its estimated increment.

The final assistant message and its resulting context state are therefore both
saved or neither is saved. A final transaction failure follows today's failed
assistant-persistence behavior: the user message remains, the completed stream
is not recorded as an assistant message, and the desktop receives an error.
Phase 2.9 later extends this same final transaction with persisted RAG sources.

Within the currently configured model/context length, state only advances from
`normal` to `warning` to `blocked`; history never shrinks. A future model-choice
spec must explicitly recompute the state when the selected model changes.

## Desktop contract and resume

Extend the session DTO returned by `ResumeStudySession` with its context usage so
the frontend renders a persisted warning or block immediately.

All new events follow Phase 2.5's session-scoped payload rule:

```text
study:context-warning
    {sessionId, usedTokens, contextLength, estimated}

study:context-limit-reached
    {sessionId, usedTokens, contextLength, estimated}

study:context-limit-unavailable
    {sessionId, message}
```

The frontend ignores events for another session. A transition event is emitted
only when the state changes; persisted resume state renders directly and does
not need a synthetic event. The unavailable notice is deduplicated in memory per
open session.

The backend checks persisted `ContextStateBlocked` before appending a new user
message. Invalid source mode validation still runs first, preserving Phase 2.5's
contract that malformed input is rejected independently of session state.

## Tasks

- [ ] `internal/domain/llm/provider.go` — `StreamResponse`, streaming metadata return, and resolved model
- [ ] `internal/domain/llm/model_catalog.go` — model metadata and catalog port
- [ ] `internal/infrastructure/openrouter/client.go` — parse/return resolved stream model and usage
- [ ] `internal/infrastructure/openrouter/model_catalog.go` — OpenRouter catalog adapter
- [ ] `internal/application/modelcatalog/service.go` — asynchronous warm-up, in-memory cache, exact-ID resolution, and on-demand refresh
- [ ] `internal/domain/study/session.go` — `ContextState`, `ContextUsage`, and blocked sentinel
- [ ] `internal/domain/study/repository.go` — persist context usage
- [ ] `internal/infrastructure/sqlite/migrations.go` — backward-compatible context columns
- [ ] `internal/infrastructure/sqlite/session_repository.go` — scan, validate, and update context usage
- [ ] `internal/application/study/` — transaction port, estimation, real-measurement replacement, state transitions, and backend send guard
- [ ] generated mocks — regenerate every changed domain/application port with Mockery
- [ ] `internal/interfaces/desktop/study.go` — session-scoped context events and resume DTO
- [ ] `main.go` — catalog adapter/cache warm-up and new study dependencies
- [ ] `frontend/src/lib/study.ts` — context DTOs and event listeners
- [ ] `frontend/src/screens/StudyChatScreen.tsx` — persistent alert, send/Enter/select blocking, and new-session action
- [ ] generated Wails bindings — regenerate after DTO and method signature changes

## Acceptance criteria

- A completed stream returns and records the same resolved model and native usage; the study service uses `InputTokens + OutputTokens`.
- The model catalog loads once asynchronously per application launch, is reused across turns, and refreshes on demand for a missing resolved model.
- Catalog failure never fails a chat response; after one failed on-demand retry, the open session shows the unavailable notice only once.
- A known occupancy below 80% is `normal`; exactly 80% is `warning`; exactly 95% is `blocked`.
- A warning is persistent, non-blocking, and restored immediately on resume.
- A blocked session disables button, Enter, and source-mode selection, and a direct backend send returns `ErrSessionContextLimitReached` before persisting a message or calling retrieval/provider ports.
- Both alert states offer a new session with the same topic and folder, no copied history or sources, default `notes` selection, and the ordinary opening turn.
- No history is truncated, deleted, or summarized automatically.
- A strict no-match turn adds `ceil(runes/3)+8` for both persisted messages; profile language does not change the formula.
- A persisted user message followed by a technical failure retains its provisional estimate and creates no assistant message.
- The next successful stream replaces accumulated estimates with native usage; missing stream usage falls back to an estimated complete request without failing the response.
- User-message/context updates and assistant-message/context updates are each atomic; a failed final transaction persists neither the assistant response nor its new context state.
- Old sessions migrate to a valid normal/unmeasured state, and malformed persisted context fields fail decoding.
- Context events carry `sessionId`; another open session ignores them.
- The RAG data cap remains independent: reaching 8,000 characters in one retrieval never by itself marks the study session warning or blocked.
