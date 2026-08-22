# Phase 2.6 — Study Context Limits

## Goal

Warn before a study session exhausts the active model's context window, then
prevent further sends before reliability collapses. The user can continue by
opening a new session in the same folder and on the same topic; Athena never
silently deletes, truncates, or summarizes the existing conversation.

This is separate from Phase 2.5's `maxContextChars = 8000`. That cap applies only
to the transient RAG data block rebuilt for one turn. This specification manages
the full model request, whose persisted conversation history grows every turn.

The send guard is deliberately reactive, not a predictive preflight. Athena
accepts a turn whenever the session's last persisted state is not `blocked`; it
does not reject a message by estimating the prospective request first. The 95%
boundary is the safety margin. A provider can still reject an accepted request
that crosses its own limit, and that rejection follows the existing error flow.

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
- makes the textarea read-only while preserving any existing draft for manual
  selection and copying;
- is enforced in `study.Service` with exported
  `study.ErrSessionContextLimitReached`, defined in `internal/domain/study`, so a
  forged desktop call cannot bypass it;
- does not prevent reading, navigating, or extracting knowledge from the
  session.

The context state is monotonic while both the resolved model ID and its context
length remain unchanged. A later native measurement can replace a larger
provisional estimate while the already-reached state remains unchanged. If the
resolved model or its catalog context length changes, recompute the state from
the new measurement and boundary; that recomputation may move in either
direction, including `warning` to `normal`.

There is no automatic history truncation, deletion, or summarization. If a
provider rejects a request before Athena has observed enough usage to warn, the
existing error flow reports it normally.

## Continue in a new session

Both warning and blocked alerts include `Start new session`. The action:

1. creates a study session in the current session's folder;
2. reuses its topic;
3. refreshes the sidebar session tree;
4. navigates to the new chat;
5. requests the ordinary opening turn through the normal keyed
   `StudyChatScreen` flow;
6. copies no draft, messages, RAG sources, summaries, or source-mode selection.

`AppShell`, which owns navigation and active-session state, coordinates the
action. A creation failure keeps the old session open. Once creation succeeds,
the new session remains even if its opening turn fails, and the ordinary error
flow allows a later retry. The action remains available while the old session is
streaming: that accepted turn continues and is persisted, while its
session-scoped events are ignored by the new screen. Disable the action only
while session creation itself is in progress so duplicate clicks cannot create
multiple sessions. The old session remains intact and resumable for reading.

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
records that same resolved model and usage through `UsageRecorder`, including
the concrete model selected by a successful free fallback.

Ignore empty model values in individual stream frames. If all non-empty model
values agree, that exact ID is the resolved model. If no non-empty ID exists or
different non-empty IDs conflict, do not invent a model: the completed response
remains successful, `StreamResponse.Model` and the corresponding recorded model
are empty, and context-length resolution is unavailable for that measurement.
Valid native usage remains usable independently of missing model metadata.

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

Native usage is usable only when `InputTokens` and `OutputTokens` are
non-negative, their integer sum does not overflow, and the sum is greater than
zero. For the fallback, apply the conservative formula independently to every
message in the actual assembled provider request—including the rebuilt system
prompt and any transient RAG system message—and to the completed assistant
response. Do not add the previous occupancy: the assembled request already
contains the complete persisted conversation history.

A `UsageRecorder` persistence failure is not missing metadata and keeps today's
failure semantics. The stream operation returns an error, the already-persisted
user message and its provisional estimate remain, and no assistant message or
final context measurement is persisted.

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

If the provisional increment changes the state to `warning` or `blocked`, emit
the transition immediately after that transaction commits and before retrieval
or provider work. The accepted turn still continues; `blocked` prevents only
subsequent turns. If its final native measurement is smaller, persist the exact
native token count with `Estimated = false` but retain the highest state already
reached while model ID and context length are unchanged.

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
    CachedContextLength(modelID string) (int, bool)
    RefreshContextLength(ctx context.Context, modelID string) (int, error)
}
```

The OpenRouter adapter implements `ModelCatalog`. An application service indexes
the returned models by exact ID and implements `ModelContextResolver`.

`CachedContextLength` is a memory-only lookup and never performs I/O.
`RefreshContextLength` performs or joins a catalog refresh and then repeats the
exact-ID lookup. This separation guarantees that the successful-response path
never accidentally waits for a network request.

Catalog validation is entry-isolated:

- an HTTP or decoding failure fails the load;
- an entry with a blank ID or non-positive context length is skipped;
- identical duplicate IDs with the same length are collapsed;
- duplicate IDs with conflicting lengths make that ID ambiguous and exclude it;
- a response with no valid entries is a failed load.

A successful validated load atomically replaces the complete cache. It does not
merge entries that disappeared from the provider response. Any failed load
preserves the previous cache unchanged.

The composition root starts one asynchronous catalog load per application
launch. Opening a session does not load the catalog. Results stay in memory for
that process. A completed stream whose non-empty resolved model is absent
triggers one on-demand refresh; no study message makes an unconditional catalog
request. Startup loads and on-demand refreshes use single-flight coordination:
at most one catalog request is in flight, and concurrent callers share its
result. If an in-flight startup load completes without the requested model, at
most one shared on-demand refresh follows.

A missing cached model never delays or endangers an already-completed response.
Persist the assistant message atomically with the new `Model`, `UsedTokens`, and
`Estimated` values, set `ContextLength` to zero, and preserve the previous
`State`. Then call `RefreshContextLength` in the background with the
application-lifetime context. Navigation does not cancel the refresh; application
shutdown does.

An empty model produced by missing/conflicting stream metadata uses the same
unresolved persistence shape (`Model = ""`, `ContextLength = 0`) but does not
start a refresh because there is no trustworthy exact ID to resolve.

Every session whose measurement is waiting on that shared refresh is evaluated
when it completes. Before applying an asynchronous result, transactionally
compare the persisted `Model`, `UsedTokens`, `ContextLength`, and `Estimated`
fields with the snapshot that initiated the refresh. If any field changed, the
result is stale and must not overwrite the newer measurement. Do not scan every
persisted session. An older unresolved session is instead reevaluated from the
cache when it is resumed.

Catalog failure never blocks chat. A startup failure alone is silent. Emit the
technical notice only when a session needs resolution and one of these
conditions remains after the applicable on-demand attempt:

- the stream supplied no trustworthy model ID;
- the refresh failed;
- the refreshed catalog still omitted the model;
- the model's catalog entry had an invalid context length.

In all cases preserve the session's last known context state and emit:

```text
Unable to determine this session's context limit.
```

The notice is transient technical feedback, not persisted context state. The
frontend displays it at most once per mounted session screen, lets the user
dismiss it, clears it after successful resolution or navigation, and may show it
once again if the unresolved session is opened later. Normal context evaluation
resumes automatically after a later successful catalog refresh.

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
not silently normalized data. Repository writes validate the same invariants
before SQL execution. `GetByID` and resume fail for a malformed session;
`ListByFolder` fails as a whole if any row is malformed rather than silently
omitting it. Newly constructed sessions explicitly initialize the same
normal/unmeasured `ContextUsage` represented by the database defaults.

The study application defines a transaction port implemented by the existing
SQLite transactor. Keep this consumer-side transaction boundary for Phase 2.6;
do not anticipate Phase 2.9 with a repository operation for sources that do not
exist yet. The SQLite session and message repositories use the existing
context-aware executor whenever they participate in one of these transactions.

The following writes are atomic:

- persisted-session read and blocked guard, user message, and provisional
  estimated increment;
- streamed assistant message plus the real/replacement context measurement;
- strict fixed assistant message plus its estimated increment.

The final assistant message and its resulting context state are therefore both
saved or neither is saved. A final transaction failure follows today's failed
assistant-persistence behavior: the user message remains, the completed stream
is not recorded as an assistant message, and the desktop receives an error.
Phase 2.9 later extends this same final transaction with persisted RAG sources.

Within the currently configured model/context length, state only advances from
`normal` to `warning` to `blocked`. The latest exact native token count is still
persisted if it is smaller than a provisional estimate; the state represents the
highest protection boundary already reached. A change in resolved model or in
that model's catalog context length recomputes the state in either direction.

### Send serialization and validation order

`study.Service` owns a process-local, non-blocking in-flight coordinator keyed by
session ID. Only one opening/study generation may run for a session; different
sessions remain independent. A competing valid call returns exported
`study.ErrStudyTurnInProgress`, defined in `internal/application/study`, before
repository, retrieval, or provider work. Release the slot on every return path;
a persisted/database lock is unnecessary for the single-process desktop
application.

`SendMessage` performs checks in this order:

1. validate source mode;
2. trim and validate message content;
3. acquire the session's in-flight slot;
4. inside the initial transaction, load the session and reject
   `ContextStateBlocked`;
5. append the user message and provisional context increment.

This preserves Phase 2.5's malformed-input precedence. The blocked read belongs
inside the same transaction as the user append, so neither a concurrent catalog
update nor another state write can race between the guard and persistence.

`RequestOpeningTurn` continues replaying an already-persisted opening message
without a provider call. If no opening message exists, it observes the same
in-flight and persisted-blocked guards before generating one. These chat guards
do not apply to knowledge extraction.

## Desktop contract and resume

Extend the session DTO returned by `ResumeStudySession` with its context usage so
the frontend renders a persisted warning or block immediately.

All new events follow Phase 2.5's session-scoped payload rule:

```text
study:context-normal
    {sessionId, usedTokens, contextLength, estimated}

study:context-warning
    {sessionId, usedTokens, contextLength, estimated}

study:context-limit-reached
    {sessionId, usedTokens, contextLength, estimated}

study:context-limit-unavailable
    {sessionId, message}
```

The frontend ignores events for another session. Emit the appropriate state
event when the state changes or when `ContextLength` changes from zero to a known
positive value, even if the preserved state stays the same. The latter clears
the unavailable notice and refreshes the measurement. A provisional transition
is emitted immediately after its initial transaction; a final transition is
emitted after the assistant/context transaction commits and before
`study:done`. Transaction failure emits neither a context transition nor
`study:done`. Persisted resume state renders directly and does not need a
synthetic event.

Application study operations receive typed, non-error-returning context-event
callbacks, following the existing source/chunk callback boundary. The desktop
adapter translates them into Wails events. A context notification can never
roll back persistence or fail a chat response; a missed event is recovered from
the persisted state on resume. Background refresh callbacks may outlive the
screen that initiated them, and session filtering makes those stale UI events
harmless.

When `ResumeStudySession` finds `ContextLength == 0`, it first checks the memory
cache for a non-empty persisted model. A hit is recomputed and persisted before
returning the DTO. A miss with a non-empty model returns the preserved state
immediately and starts or joins the background refresh. An empty model returns
the preserved state and emits the unavailable notice without attempting a
refresh. Resume never makes an LLM call and never waits for catalog I/O.

The backend checks persisted `ContextStateBlocked` before appending a new user
message. Invalid source mode validation still runs first, preserving Phase 2.5's
contract that malformed input is rejected independently of session state.

Extend `StudyErrorEvent` with a stable code. At minimum,
`context_limit_reached` and `turn_in_progress` identify failures that occur
before user-message persistence. If one races with the frontend's optimistic
append, the screen removes that bubble and restores the draft. Retrieval or
provider errors retain the optimistic bubble because the user message is already
durable.

## Tasks

- [ ] `internal/domain/llm/provider.go` — `StreamResponse`, streaming metadata return, and resolved model
- [ ] `internal/domain/llm/model_catalog.go` — model metadata and catalog port
- [ ] `internal/infrastructure/openrouter/client.go` — parse/return resolved stream model and usage
- [ ] `internal/infrastructure/openrouter/model_catalog.go` — OpenRouter catalog adapter
- [ ] `internal/application/modelcatalog/service.go` — asynchronous warm-up, in-memory cache, exact-ID resolution, and on-demand refresh
- [ ] `internal/domain/study/session.go` — `ContextState`, `ContextUsage`, and blocked sentinel
- [ ] `internal/domain/study/repository.go` — persist context usage
- [ ] `internal/infrastructure/sqlite/migrations.go` — backward-compatible context columns
- [ ] `internal/infrastructure/sqlite/session_repository.go` and `message_repository.go` — context-aware transaction execution, scan/write validation, and context updates
- [ ] `internal/application/study/` — transaction port, typed context callbacks, estimation, real-measurement replacement, state transitions, async-refresh compare-and-set, per-session in-flight coordination, and backend guards
- [ ] generated mocks — regenerate every changed domain/application port with Mockery
- [ ] `internal/interfaces/desktop/study.go` — session-scoped context/error events, callbacks, and resume DTO
- [ ] `main.go` — catalog adapter/cache warm-up and new study dependencies
- [ ] `frontend/src/lib/study.ts` — context DTOs and event listeners
- [ ] `frontend/src/components/app-shell.tsx`, `frontend/src/components/study-folder-tree.tsx`, and `frontend/src/screens/StudyChatScreen.tsx` — coordinated session creation/tree refresh, persistent alerts, transient unavailable feedback, optimistic-error reconciliation, read-only blocked composer, and new-session action
- [ ] generated Wails bindings — regenerate after DTO and method signature changes

## Acceptance criteria

- A completed stream with consistent model metadata returns and records the same resolved model and native usage; blank/conflicting model metadata remains a successful turn with an empty model, and the study service uses valid `InputTokens + OutputTokens`.
- Native usage requires non-negative values, a positive overflow-safe sum, and otherwise falls back to a per-message estimate of the complete assembled request and response.
- A `UsageRecorder` write failure retains the existing failed-stream behavior rather than being treated as missing metadata.
- The model catalog loads once asynchronously per application launch, is reused across sessions/turns, coalesces concurrent loads, validates entries in isolation, atomically replaces a valid cache, and refreshes in the background for a missing resolved model.
- Catalog lookup or refresh never delays or fails a completed chat response; unresolved measurements persist with zero context length, successful late refreshes update only matching non-stale measurements, and a relevant unresolved session shows the transient unavailable notice only once per mounted screen.
- A known occupancy below 80% is `normal`; exactly 80% is `warning`; exactly 95% is `blocked`.
- State is monotonic for an unchanged model/limit even when a real measurement replaces a larger estimate; a resolved-model or catalog-length change recomputes it in either direction.
- A warning is persistent, non-blocking, and restored immediately on resume; a cache-resolvable previously unknown limit is persisted before the resume DTO returns.
- A blocked session makes the textarea read-only, disables button, Enter, and source-mode selection, and a direct backend send returns `study.ErrSessionContextLimitReached` from `internal/domain/study` before persisting a message or calling retrieval/provider ports.
- A second concurrent turn for one session returns `ErrStudyTurnInProgress` without waiting or doing repository/provider work, while different sessions remain independent.
- Invalid source mode and blank content validation precede in-flight and persisted-state guards; the blocked read, user append, and provisional increment share one transaction.
- A turn accepted below 95% continues after its provisional increment reaches `blocked`; its state event is emitted immediately after commit and blocks only later turns.
- Both alert states offer a new session with the same topic and folder, no copied draft/history/sources, default `notes` selection, and the ordinary opening turn; creation failure stays on the old session, while opening failure keeps the new session.
- No history is truncated, deleted, or summarized automatically.
- A strict no-match turn adds `ceil(runes/3)+8` for both persisted messages; profile language does not change the formula.
- A persisted user message followed by a technical failure retains its provisional estimate and creates no assistant message.
- The next successful stream replaces accumulated estimates with native usage; missing stream usage estimates every assembled request message—including system/RAG messages—and the assistant response without failing the response.
- User-message/context updates and assistant-message/context updates are each atomic; a failed final transaction persists neither the assistant response nor its new context state.
- Async refresh updates are compare-and-set against the initiating measurement and cannot overwrite newer context state.
- Old sessions migrate to a valid normal/unmeasured state; malformed persisted context fields fail reads and writes, and one malformed row fails a complete folder listing.
- Context events carry `sessionId`; another open session ignores them. Normalization and newly available limits emit state events, final state events precede `study:done`, and event-delivery failure never changes persisted results.
- Stable error-event codes let the frontend remove and restore only optimistic messages rejected before persistence.
- The RAG data cap remains independent: reaching 8,000 characters in one retrieval never by itself marks the study session warning or blocked.
