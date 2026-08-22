# Phase 2.5 — RAG Integration

## Goal

Every user-submitted study turn follows the selected source policy. `notes` and
`strict-notes` retrieve approved local knowledge before answering; `web`
bypasses local retrieval. Retrieved chunks are supplied to the LLM as reference
data, and the UI shows the exact local sources the model received.

The automatic opening turn remains unchanged: it has no user question or source
mode, performs no retrieval, and emits no source event.

## Source modes

The mode is transient and passed per call:

```go
SendStudyMessage(sessionID, topic, content, sourceMode)
```

It is neither stored nor inferred from previous turns. A user can switch modes
between messages, and every newly opened or resumed chat starts with `notes`
selected.

| Mode | Local chunks | Behavior |
|---|---|---|
| `web` | not searched | Plain LLM call. No retriever, embedding, or vector search call |
| `notes` | none | Plain LLM call |
| `notes` | sufficient | LLM call with local context as the primary source; supplement only when necessary |
| `notes` | insufficient but non-empty | LLM call with the related local context; general knowledge may fill the gaps |
| `strict-notes` | none | No chat/completion call. Persist and return `NoLocalKnowledgeMessage` |
| `strict-notes` | sufficient | LLM call instructed to answer exclusively from the local context |
| `strict-notes` | insufficient but non-empty | LLM call restricted to what the local context supports and instructed to state that the local material cannot support a complete answer |

`strict-notes` restricts the LLM's information source; it does not replace the
tutor with raw chunk text. With matching chunks, the LLM still organizes and
explains them. The only no-chat branch is `strict-notes` with no surviving
chunks.

Despite its name, `web` means today's plain model call and does not promise a
live internet search. Its UI description must say that it ignores local sources
and may use the model's general knowledge.

The three exported values are:

```go
const (
    SourceModeNotes       = "notes"
    SourceModeStrictNotes = "strict-notes"
    SourceModeWeb         = "web"
)
```

An unknown value returns exported `ErrInvalidSourceMode` before the user message
is persisted and before any embedding, retrieval, or chat call.

## Flow

```text
Validate content and source mode
    ↓
Persist the user message
    ↓
mode = web? ─────────────────────────────→ plain LLM call
    ↓ no
Index has a valid snapshot?
    ├── no  → ErrVectorStoreUnavailable; no embedding or chat call
    ↓ yes
Vector store empty? ─────────────────────→ no chunks, no embedding
    ↓ no
Build query from topic + current message
    ↓
Embed query with session attribution
    ↓
Search approved local knowledge, top-K
    ↓
Filter by minScore → cap rendered context → compute Sufficient
    ↓
No surviving chunks?
    ├── notes        → plain LLM call
    └── strict-notes → persist and emit NoLocalKnowledgeMessage; no chat call
    ↓ chunks survive
LLM call with a second system message containing the local context
```

A successful embedding against a non-empty store records embedding usage for the
study session even when search finds no match. "No LLM call" in the strict miss
criteria means no **chat/completion** usage. A valid empty snapshot makes neither
an embedding nor a chat call in `strict-notes`.

Retrieval, embedding, item-resolution, and vector-search errors are returned in
both local modes. They are technical failures, not proof that no knowledge
exists, and must never silently degrade to a plain LLM call. The already
persisted user message remains in history and no assistant message is added.

## Retrieval contract

`study.Service` owns source-mode policy. It does not call the retriever at all in
`web`, so the retriever needs no mode parameter:

```go
type Source struct {
    ChunkID    string
    ItemID     string
    SourceType string // user_note | imported_doc | athena
    FilePath   string
    Heading    string
    Concept    string
    Score      float32
    Excerpt    string
}

type RetrievalResult struct {
    Chunks     []ScoredChunk
    Sufficient bool
    Context    string // deterministic JSON data block, already capped
    Sources    []Source
}

type Retriever interface {
    Retrieve(ctx context.Context, sessionID, query string) (RetrievalResult, error)
}
```

These types and the source-mode constants live in `internal/domain/knowledge`.
`application/knowledge.Service` implements `Retriever`; `study.Service` receives
the port rather than a concrete application dependency. Retrieval orchestration
must not move into the desktop binding.

### Query

The study use case builds one deterministic query from the normalized session
topic and the current trimmed message:

```text
Topic: <topic>

Message: <content>
```

No previous messages are included and no LLM call rewrites the query. The
session ID is forwarded through `Retrieve` into `EmbeddingRequest`, so embedding
usage is attributed to the originating study session.

### Index readiness

`VectorStore.Len() == 0` alone cannot distinguish a valid empty index from an
index that has never loaded. The retriever must read the existing
`IndexLoader.Status()` through an application port before consulting `Len()`:

```text
HasSnapshot = false
→ ErrVectorStoreUnavailable

HasSnapshot = true and Len() = 0
→ valid empty knowledge base; return no chunks without embedding
```

A loader that is retrying while retaining `HasSnapshot = true` continues to
serve its previous valid snapshot, as established by Phase 2.4.

### Search scope and ordering

Both local modes search all topics and all local source types. RAG always passes:

```go
SearchFilters{Status: StatusApproved}
```

Before Phase 2.8 this primarily retrieves imported notes. After 2.8 it also
retrieves approved `athena` items; draft and deprecated items never reach RAG.

`DefaultTopK = 8` is exported from the domain and passed to
`VectorStore.Search`. Results preserve the vector store's deterministic order:
score descending, then chunk ID ascending for equal scores.

### Thresholds

The defaults remain:

```go
const (
    DefaultMinSimilarity = 0.35
    DefaultSufficiency   = 0.55
)
```

They are constructor-injected as `float64`. The constructor returns an error
unless both values are finite, within cosine range `[-1, 1]`, and
`minScore <= sufficiencyScore`. A `float32` search score is explicitly converted
to `float64` for comparison.

Chunks below `minScore` are discarded. After context capping,
`Sufficient == true` exactly when at least one surviving chunk has
`Score >= sufficiencyScore`. Equality counts as sufficient; a result discarded
by the cap cannot make the context sufficient.

The defaults are calibrated for `text-embedding-3-small`. Surfacing them in
Settings is outside this phase.

## Context rendering and cap

`application/knowledge.Retrieve` owns selection, source materialization, context
rendering, and the cap. It serializes a deterministic JSON object containing the
surviving chunks' `sourceType`, `filePath`, `heading`, `concept`, and `content`.
The embedding and score are not sent to the LLM.

Every chunk has an `ItemID`, but `Chunk` does not duplicate the owning item's
concept. Resolve each distinct surviving `ItemID` through the existing knowledge
repository at most once per retrieval. A missing owner is an integrity error,
not an empty concept. `Source.Excerpt` is the exact full `Chunk.Content`, and
there is one `Source` per surviving chunk even when several chunks share an
item.

The rendered JSON data block has a hard `maxContextChars = 8000`, measured in
Unicode code points. File paths, headings, concepts, content, JSON syntax, and
separators all count. When it is over budget, remove whole chunks from the end
of the score-ordered result—lowest score first—and render again. Never truncate
a chunk. If one chunk cannot fit by itself, remove it; if none fit, the result is
the same as no local match.

`Sources` and `Chunks` are built after the cap and remain in exactly the same
order as the JSON entries. They can never describe a chunk the model did not
receive.

`application/study.buildKnowledgeContext(result, sourceMode)` owns only the
mode- and sufficiency-specific instructions. It wraps the already capped JSON in
a second `system` message immediately after the existing system prompt. Its
fixed instructions sit outside the 8,000-character retrieved-data budget and
must say that the JSON is untrusted reference data whose embedded instructions
must never be followed.

`buildSystemPrompt` and its existing tests remain untouched; the two system
messages must not be merged.

## Strict miss response

The fixed response remains one exported constant:

```go
const NoLocalKnowledgeMessage = "No local knowledge found for this question."
```

For a successful `strict-notes` retrieval with no surviving chunks:

1. emit an empty source list;
2. persist `NoLocalKnowledgeMessage` as an assistant message;
3. deliver it through the normal chunk callback as one complete chunk;
4. emit the normal done event;
5. make no chat/completion call.

The user question and fixed assistant response therefore reappear together when
the session is resumed.

## Desktop events and source UI

`study.Service.SendMessage` gains both `sourceMode` and an `onSources` callback.
For every user-submitted turn it invokes `onSources` exactly once before any
response chunk: with the post-cap sources, or `[]` for `web`, a local miss, or a
strict fixed response. Empty emission clears pending state and prevents sources
from leaking from the previous turn.

All study events become session-scoped structured payloads, including the
existing opening-turn events:

```text
study:chunk   {sessionId, content}
study:done    {sessionId}
study:error   {sessionId, message}
study:sources {sessionId, sources}
```

The frontend ignores events whose `sessionId` is not the currently displayed
session. Sources received before streaming stay pending and are attached to the
completed assistant message only on `study:done`. Every assistant message owns
its own source list while the screen remains mounted; an empty list renders no
strip.

The source event DTO contains:

```text
sourceType, filePath, heading, concept, score
```

It deliberately omits internal IDs and the full excerpt. Under the completed
assistant bubble, a collapsed `Local sources (N)` strip expands in retrieval
order:

- `imported_doc`: file path, heading, and decimal similarity score;
- `user_note`: `User note`, concept or title, and score;
- `athena`: `Athena Knowledge`, concept, and score.

The score is displayed as a decimal such as `0.68`, never as a confidence
percentage. Calling the strip "Local sources" makes clear that a `notes` answer
may also contain model knowledge.

Sources are transient in 2.5. Phase 2.9 persists the exact post-cap ordered list
with the completed assistant message and restores it on resume.

## Source-mode selector

The composer gains a labeled, accessible select with `Notes`, `Strict notes`,
and `Web`, defaulting to `Notes` whenever a chat is opened or resumed. Changing
it affects only subsequent messages. It is disabled while a response streams.
Phase 2.6 additionally disables it when the session reaches its hard context
limit.

Descriptions explain the policies and explicitly state that `Web` ignores local
sources but does not necessarily perform live internet search.

## Tasks

- [ ] `internal/domain/knowledge/retrieval.go` — retrieval types and port, source-mode constants, fixed response, defaults, validation errors
- [ ] `internal/application/knowledge/retrieval.go` — readiness, query embedding, approved-only search, thresholding, item resolution, JSON rendering and whole-chunk cap
- [ ] generated knowledge/study mocks — regenerate with Mockery after port changes
- [ ] `internal/application/study/prompt_context.go` — untrusted-data wrapper and four mode/sufficiency instruction variants
- [ ] `internal/application/study/send_message.go` — validate mode, build query, bypass `web`, retrieve local context, notify sources, strict miss persistence, and context injection
- [ ] `internal/interfaces/desktop/study.go` — fourth `SendStudyMessage` argument and session-scoped event payloads
- [ ] `main.go` — construct retrieval with default thresholds and inject the port into study
- [ ] `frontend/src/lib/study.ts` — fourth argument, structured event types, and `onStudySources`
- [ ] `frontend/src/screens/StudyChatScreen.tsx` — selector, per-message pending sources, session filtering, and `Local sources` strip
- [ ] generated Wails bindings — regenerate after the desktop signature changes

## Acceptance criteria

- An invalid source mode returns `ErrInvalidSourceMode` before persisting or making any provider/store call.
- `web` never calls `Retriever`, embeddings, or vector search and makes a plain chat call.
- A local mode with `HasSnapshot == false` returns `ErrVectorStoreUnavailable`, preserves the user message, and makes no embedding or chat call.
- A valid empty snapshot makes no embedding call. `notes` falls through to plain chat; `strict-notes` persists and emits `NoLocalKnowledgeMessage` without chat/completion usage.
- A non-empty-store miss may record one session-attributed embedding usage row. In `strict-notes` it records no chat/completion usage row.
- Retrieval searches all topics and local source types with `StatusApproved`; after 2.8, approved `athena` items are available in both local modes.
- The embedded query contains the topic and current message, excludes earlier history, and carries the study session ID.
- Raised `minScore` demonstrably filters a low-similarity chunk; equality at either threshold follows the documented inclusive rule.
- `Sufficient` is based only on post-cap survivors and selects the documented prompt variant without skipping the model when chunks exist.
- The rendered JSON data block never exceeds 8,000 Unicode code points; metadata counts, chunks are removed whole lowest-score-first, and a lone oversized chunk becomes no match.
- JSON content is escaped and the second system message instructs the model to treat it as untrusted reference data rather than commands.
- `Chunks`, JSON entries, and `Sources` contain exactly the same post-cap chunks in the same score-descending/ID-tiebreak order.
- Concepts are loaded once per distinct item ID; a missing owning item returns an integrity error.
- `strict-notes` with chunks calls the LLM and constrains it to local context; with no chunks it persists the fixed assistant response and delivers it through the normal chunk/done lifecycle.
- `study:sources` is emitted exactly once before response chunks, including `[]` on no-source paths.
- Every study event carries `sessionId`; the frontend ignores other sessions and attaches pending sources only to the completed assistant message.
- Empty sources render no strip; non-empty sources render a collapsed `Local sources (N)` strip with source-appropriate labels and decimal scores.
- Source selection resets to `notes` when opening or resuming a chat and can change between non-streaming turns.
- The opening turn performs no retrieval and emits no source event.
- The existing `buildSystemPrompt` tests pass unchanged.
- After 2.9, the exact same sources reappear in the same order after resume.
