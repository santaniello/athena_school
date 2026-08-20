# Phase 2.4 — Vector Search

## Goal

Pure-Go in-process vector search that retrieves the most relevant current
knowledge chunks by cosine similarity, stays coherent with SQLite mutations,
and becomes available through an explicit background-loading lifecycle. No
external vector database and no CGO dependency are introduced.

## Domain contracts

```go
type SearchFilters struct{ Topic, Source, Status string } // "" = no constraint

type ScoredChunk struct {
    Chunk Chunk
    Score float32
}

type VectorStore interface {
    Add(ctx context.Context, chunks []Chunk) error
    ReplaceAll(ctx context.Context, chunks []Chunk) error
    Search(ctx context.Context, query []float32, topK int, filters SearchFilters) ([]ScoredChunk, error)
    Remove(ctx context.Context, ids []string) error
    Len() int
}
```

`ReplaceAll` is required by the background loader. It builds and validates a
complete replacement snapshot away from the active slice and publishes it in
one atomic swap. A failed initial load publishes nothing; a failed retry keeps
the previously usable snapshot intact. Passing an empty valid snapshot clears
the store and marks a successfully loaded, genuinely empty knowledge base.

The domain exposes typed sentinels so callers and tests do not depend on error
text:

```go
var (
    ErrInvalidTopK          = errors.New("topK must be greater than zero")
    ErrInvalidVector        = errors.New("vector must be non-empty, finite, and have non-zero norm")
    ErrInvalidChunkID       = errors.New("chunk id is required")
    ErrDuplicateChunkID     = errors.New("chunk ids must be unique within a batch")
    ErrUnknownSource        = errors.New("unknown knowledge source")
    ErrVectorStoreUnavailable = errors.New("vector store is unavailable")
)
```

Existing `ErrUnknownStatus` remains the status-validation sentinel. Error text
is internal and English; callers use `errors.Is`.

### Why the interface differs from the original sketch

- **`[]ScoredChunk` instead of `[]Chunk`** — 2.5's sufficiency threshold and
  source footer depend on the similarity score; hiding it would force a second
  calculation.
- **`Remove`** — a successful delete or replacement must make the previous
  chunks unreachable without waiting for a restart.
- **`Len`** — 2.5 can avoid a paid embedding call when the *ready* store is
  genuinely empty. Readiness must be checked separately: `Len() == 0` never
  means that a loading or failed store is empty.
- **`ReplaceAll`** — background load and retry need an atomic snapshot publish;
  repeated `Add` calls cannot remove rows that disappeared from SQLite.

## In-memory store semantics

### Memory shape and ownership

The implementation remains a flat contiguous slice protected by
`sync.RWMutex`:

```go
struct {
    mu     sync.RWMutex
    chunks []knowledge.Chunk
    ready  bool
}
```

No per-topic or approximate-nearest-neighbor index is introduced. Ten thousand
1536-dimensional chunks remain comfortably within the brute-force budget.

The store owns every embedding it holds:

- `Add` and `ReplaceAll` deep-copy the caller's embedding before normalizing
  it, so the provider's raw vector remains unchanged for SQLite persistence.
- `Search` returns a deep copy of each result, including its embedding, so a
  caller cannot mutate the store outside its mutex.
- `ScoredChunk.Chunk.Embedding` is the normalized in-memory vector, not the raw
  provider vector. Consumers must not persist a search result as the canonical
  raw embedding; SQLite remains that source of truth.

### Validation and atomic mutation

A chunk is valid for indexing only when:

- `strings.TrimSpace(chunk.ID) != ""`; IDs are validated, never trimmed or
  rewritten;
- its source is one of `SourceAthena`, `SourceUserNote`, or
  `SourceImportedDoc`;
- its status is one of `StatusDraft`, `StatusApproved`, or
  `StatusDeprecated`;
- its topic and owning `ItemID` satisfy the existing domain invariants;
- its embedding is non-empty, every component is finite, and its Euclidean
  norm is non-zero.

Norm accumulation uses sufficient precision to avoid float32 overflow during
validation. `NaN`, positive infinity, and negative infinity are invalid even
when the computed norm would otherwise be non-zero.

`Add` validates and normalizes the complete batch before changing the active
slice. One invalid chunk rejects the entire batch. IDs must be unique *within*
one call; two occurrences of the same ID return `ErrDuplicateChunkID` and
apply nothing. Across separate calls, `Add` is an upsert by ID: an existing
chunk is replaced without increasing `Len`, and repeating the same call is
idempotent.

`ReplaceAll` applies the same validation, duplicate-ID, normalization, copying,
and all-or-nothing rules before atomically replacing the active snapshot. A
new store starts unavailable (`ready == false`); the first successful
`ReplaceAll` sets `ready`, including when the replacement slice is empty.
`Add` and `Remove` do not turn a never-loaded store into a ready store.

`Remove` validates all supplied IDs before mutation, removes every matching ID
atomically, and is idempotent: valid unknown IDs are successful no-ops. Empty
`Add` and `Remove` calls are successful no-ops, even with a canceled context.

For non-empty work, error precedence is deterministic:

1. `ctx.Err()`
2. structural errors (`ErrInvalidTopK`, `ErrInvalidChunkID`,
   `ErrDuplicateChunkID`, unknown source/status)
3. `ErrInvalidVector`
4. processing errors

`Add`, `ReplaceAll`, `Search`, `Remove`, and `Len` are safe for concurrent use.
The context is checked before work and between chunks during long loops. A
canceled `Add`, `ReplaceAll`, or `Remove` applies no partial mutation; a
canceled `Search` returns no partial result. An empty `ReplaceAll` is a real
snapshot publication, not a no-op, so a canceled empty replacement returns the
context error and preserves the previous state.

### Cosine search

`Add`/`ReplaceAll` normalize stored vectors to unit length. `Search` copies and
normalizes the query once, without mutating the caller's slice. Cosine then
reduces to a dot product because both operands are unit vectors; normalizing
only the stored side would scale every score by the query norm and would not be
cosine similarity.

`Search` executes in this order:

1. reject `topK <= 0` with `ErrInvalidTopK` after the context check;
2. reject an invalid query with `ErrInvalidVector`;
3. return `ErrVectorStoreUnavailable` when no snapshot has ever been
   published;
4. skip a chunk on any set-and-mismatched filter before touching its vector;
5. skip a stored vector whose dimension differs from the query;
6. calculate the dot product;
7. order by `Score` descending, then by `Chunk.ID` ascending;
8. truncate to at most `topK`.

Topic, source, and status filters use exact Go string equality. Only `""`
means “no constraint.” The vector store never trims, folds case, or performs
Unicode normalization. Knowledge write boundaries call the domain topic
normalizer, which applies `strings.TrimSpace` and rejects the empty result while
preserving case, accents, and internal whitespace. Therefore `Go` and `go`
remain different topics in this phase. Canonical case-insensitive topic
identity, display-name preservation, and migration of existing rows require a
separate specification and grilling before implementation.

Scores are returned regardless of whether they are low or negative. Phase 2.5,
not the storage adapter, owns retrieval thresholds.

## Safe loading from SQLite

### `ListCurrent`, never `ListAll`

Startup uses `chunks.ListCurrent(ctx, llm.EmbeddingModel)`. The old task text
that named `ListAll` was incorrect: loading every persisted vector would
violate the model/version safety acceptance criterion.

The repository returns a load report rather than losing per-row diagnostics:

```go
type ChunkLoadIssue struct {
    ChunkID string
    ItemID  string
    Source  string
    FilePath string
    Reason  string // stable reason code; UI maps it to user-facing English
}

type ChunkLoadResult struct {
    Chunks []Chunk
    Issues []ChunkLoadIssue
}

type ChunkRepository interface {
    SaveAll(ctx context.Context, chunks []Chunk) error
    ListAll(ctx context.Context) ([]Chunk, error)
    ListCurrent(ctx context.Context, embeddingModel string) (ChunkLoadResult, error)
    DeleteByFilePath(ctx context.Context, path string) ([]string, error)
    DeleteByItemID(ctx context.Context, itemID string) ([]string, error)
    UpdateMetadataByItemID(ctx context.Context, itemID, topic, status string) ([]Chunk, error)
}
```

`ListCurrent` applies these rules:

- a different embedding model is excluded; it is expected reindex work, not a
  corrupt-row warning;
- every chunk must have an existing owning `knowledge_items` row;
- chunk and Item source, topic, and status must match;
- Athena chunks additionally require non-null `item_updated_at` equal to the
  Item's `updated_at`;
- imported-note chunks deliberately do not compare `item_updated_at`; their
  content freshness remains governed by `ingested_files.mtime` and embedding
  model;
- malformed embedding blobs, invalid IDs/vectors, missing Items, metadata
  mismatches, and unknown source/status values are excluded and reported as
  `ChunkLoadIssue`s;
- a query, iteration, or other database-wide failure returns an error for the
  entire load rather than pretending the database is empty.

Wrong-model and stale Athena chunks remain out of retrieval until the existing
re-import flow or 2.8's consent-based backfill repairs them. Unknown values are
never guessed: a database created by a future version degrades with a visible
warning instead of letting the current version search semantics it does not
understand.

The loader validates repository output again before `ReplaceAll`. Invalid rows
are partitioned out, their issues are combined with repository decode issues,
and the complete valid subset is published in one atomic swap. Thus one corrupt
chunk does not make the other 2,999 valid chunks unavailable.

## Index lifecycle and desktop UX

Loading begins in the background after Wails starts, so the window can render
before SQLite chunks are decoded and normalized. The current claim that a
pre-`wails.Run` synchronous load is covered by the React splash is false and is
removed by this spec.

The application owns a concurrency-safe index coordinator with these externally
observable states:

```go
type IndexState string

const (
    IndexStateLoading           IndexState = "loading"
    IndexStateReady             IndexState = "ready"
    IndexStateReadyWithWarnings IndexState = "ready_with_warnings"
    IndexStateFailed            IndexState = "failed"
)

type IndexStatus struct {
    State       IndexState
    HasSnapshot bool
    Issues      []ChunkLoadIssue
    LastError   string // safe summary; full technical error is logged
}
```

Status also reports whether a previously published snapshot exists. That flag
distinguishes an initial load from a retry that can keep serving the old
snapshot. A retry sets `State = loading` while preserving `HasSnapshot`; if it
fails with a prior snapshot, the coordinator restores the preceding ready
state, keeps the snapshot, and populates `LastError` rather than mislabeling
working search as globally failed.

### Initial load

1. The window opens and renders `Loading knowledge index...`.
2. The entire application remains behind this loading screen; no search or
   knowledge mutation can race the initial snapshot.
3. A successful load with no row issues becomes `ready`, including when the
   valid snapshot is empty.
4. A successful load that isolated rows becomes `ready_with_warnings`; the app
   opens and local search uses the valid subset.
5. A database-wide failure becomes `failed` and renders:

```text
Knowledge index could not be loaded.

[Retry] [Continue without local search]
```

`Continue without local search` opens the rest of the app with a persistent
warning. A failed/unpublished index is unavailable, not empty: local-search
entry points return/wrap `ErrVectorStoreUnavailable` and must never translate
the condition to “No local knowledge found.”

### Status delivery and recovery

The frontend first registers `knowledge-index:status` event listeners and then
calls `GetKnowledgeIndexStatus()`. The initial query closes the race where a
fast background load finishes before React subscribes; continuous polling is
not used.

`RetryKnowledgeIndex()` rebuilds a separate snapshot. While retry is running:

- navigation and unrelated application features remain available;
- search continues using the previous valid snapshot when one exists;
- local search remains unavailable when no snapshot has ever been published;
- import, edit, approve/deprecate, and delete knowledge mutations are rejected
  by a backend guard and disabled in the UI, preventing a retry snapshot from
  overwriting a concurrent change.

A successful retry atomically replaces the active snapshot. A failed retry
keeps the prior snapshot and status usable, adds the retry failure to the
visible warning, and never clears working search data.

`ready_with_warnings` shows the affected count and a `Review` action. The review
view identifies affected content using safe fields (`ChunkID`, `ItemID`,
`Source`, and `FilePath`) and maps stable reason codes to understandable English
copy. It directs imported notes to re-import their folder and, once 2.8 exists,
Athena items to reindex. Full technical errors remain in logs.

## Keeping memory coherent after mutations

SQLite is the durable source of truth, but every already-available mutation
that affects searchable imported-note chunks is wired in this phase. Deferring
delete/deprecate/topic synchronization to 2.8 would knowingly ship stale RAG
between 2.4 and 2.8.

### Re-import and delete

`DeleteByFilePath` and `DeleteByItemID` return the exact chunk IDs removed by
their transaction. Callers retain those IDs but call `store.Remove` only after
the surrounding transaction commits. A rolled-back transaction never evicts
the still-current memory entries.

For a changed imported file:

1. read, chunk, embed, and validate every replacement before deleting anything;
2. in one SQLite transaction, delete the old chunks while capturing their IDs,
   insert the replacements, update the shadow Item, and update
   `ingested_files`;
3. after commit, remove the captured IDs from memory;
4. add the new chunks.

The old memory entries are removed before the replacements are added, so an
`Add` failure can temporarily omit content but can never keep serving stale
content. The application uses a short internal post-commit reconciliation
context rather than abandoning mandatory in-memory cleanup merely because the
request context was canceled after SQLite committed.

Deleting an Item follows the same commit-then-remove rule. Unknown/already
removed IDs remain harmless because `Remove` is idempotent.

### Metadata-only changes

Status and topic changes update the Item and its persisted chunk metadata in
the same SQLite transaction. `UpdateMetadataByItemID` returns the updated
chunks; after commit, `store.Add` upserts those metadata versions without a new
embedding call.

- approve/deprecate changes status only;
- moving an imported shadow Item to another topic changes topic only;
- editing a shadow Item's concept/definition does not change its raw imported
  chunk embeddings — the file remains the searchable source of truth and a
  future re-import regenerates/overwrites the shadow fields;
- changed file content creates new embeddings.

### Post-commit indexing failure

An in-memory update cannot roll back an already committed SQLite transaction.
If post-commit `Remove`/`Add` reconciliation fails:

- the durable mutation remains a success;
- stale chunks are never deliberately restored;
- the use case returns the persisted result plus a typed indexing warning;
- desktop bindings log the technical failure but report the durable operation
  as successful;
- the persistent warning offers `Retry indexing`, whose full atomic reload
  self-heals from SQLite; restarting performs the same load.

Import summaries distinguish a persisted file from its index warning. They
must not call the durable import a failure, because `ingested_files` now records
the current mtime/model and a repeated import would legitimately skip it.

## Performance verification

Budget check: 10k × 1536 float32 multiply-adds is approximately 15.4 MFLOP,
plus ordering 10k scored candidates. The product target remains under 500 ms on
a documented modern reference machine, but shared CI timing is not a stable
quality gate.

Verification therefore consists of:

- a normal functional test that adds 10,000 1536-dimensional chunks, searches
  them, and asserts count/order/correctness without a wall-clock assertion;
- `BenchmarkSearch10Kx1536`, reporting time and allocations for regression
  comparison;
- no hard 500 ms assertion in CI.

The store and loader are unit-testable without a real database; repository
filtering has SQLite integration tests separately.

## ADR-004

This spec ships with `specs/decisions/ADR-004-local-vector-store.md` recording:

1. in-process brute-force cosine over a flat slice, pure Go, with SQLite as the
   durable source of truth;
2. rejected alternatives: `sqlite-vec` (CGO),
   `philippgille/chromem-go` (duplicate persistence), and hosted vector
   databases (not local-first);
3. `openai/text-embedding-3-small` (1536d) inherited from
   `llm.EmbeddingModel`, raw float32 little-endian persistence, and the rule
   that changing the embedding model requires re-ingestion/reindexing;
4. normalization, defensive copying, upsert-by-ID, deterministic ties, and
   atomic snapshot replacement;
5. background loading, explicit readiness/failure states, partial-row
   isolation, and retry behavior;
6. the memory profile and the benchmark-based performance gate;
7. amendment of ADR-002: `internal/infrastructure/vectorstore` joins
   `make mutation-go`, while thin SQLite and HTTP adapters remain excluded.

## Tasks

- [ ] `internal/domain/knowledge/vectorstore.go` — port, scored/filter types,
      validation helpers, index status/issue types, and typed errors
- [ ] `internal/domain/knowledge/chunk.go` — `ChunkLoadResult`,
      `ListCurrent`, delete methods returning removed IDs, and the metadata
      update contract
- [ ] `internal/domain/knowledge/item.go` and knowledge write boundaries —
      centralize edge-trimmed, non-empty Topic normalization without case
      folding
- [ ] `internal/infrastructure/vectorstore/store.go` — atomic batch validation,
      deep copies, normalization, upsert, atomic `ReplaceAll`, concurrent
      safety, cancellation, exact filters, deterministic ordering, and
      idempotent removal
- [ ] `internal/infrastructure/vectorstore/store_test.go` — Given/When/Then
      unit tests plus the 10,000-chunk functional case and
      `BenchmarkSearch10Kx1536`
- [ ] `internal/infrastructure/sqlite/chunk_repository.go` — safe
      `ListCurrent`, per-row issue isolation, deleted-ID returns, and atomic
      metadata updates
- [ ] `internal/application/knowledge/index_loader.go` — background lifecycle,
      status, issue aggregation, atomic publish/retry, mutation guard, and
      typed durable-write/index-warning policy
- [ ] `internal/application/ingest/service.go` /
      `internal/application/ingest/import_folder.go` — inject the store,
      validate before persistence, capture old IDs, and reconcile memory after
      commit without misreporting durable success
- [ ] `internal/application/knowledge/approve.go`, `deprecate.go`, `update.go`,
      and `delete.go` — keep existing imported-note chunk metadata/deletions
      coherent with memory; 2.8 still owns Athena item embedding and backfill
- [ ] `internal/interfaces/desktop` / `main.go` — start the loader from Wails'
      `OnDomReady` lifecycle so the window can render; expose
      `GetKnowledgeIndexStatus` and
      `RetryKnowledgeIndex`; emit `knowledge-index:status`; log indexing
      warnings while preserving durable-operation success
- [ ] frontend boot/app shell — register the event before the initial status
      query; render `Loading knowledge index...`, failure choices, persistent
      unavailable/partial warnings, `Review`, and mutation disabling during
      retry
- [ ] `main.go` — construct and wire the store/coordinator without a
      pre-`wails.Run` synchronous chunk load
- [ ] regenerate `internal/domain/knowledge/mocks/` with `make mock`
- [ ] `specs/decisions/ADR-004-local-vector-store.md`
- [ ] `Makefile`, mutation documentation, and ADR cross-references — add
      `internal/infrastructure/vectorstore` to the Go mutation loop
- [ ] `README.md` — document SQLite + in-memory vector ownership, background
      readiness/recovery, and remove the obsolete `~/.athena/vectors/` claim
- [ ] `CHANGELOG.md` — add the implementation entry under `[Unreleased]`

## Acceptance Criteria

- `Search` returns at most `topK` results with exposed scores, ordered by score
  descending and then `Chunk.ID` ascending
- exact topic/source/status filters restrict results; `""` alone means no
  constraint, and all knowledge write paths edge-trim/reject empty topics in
  the domain without folding case
- `topK <= 0`, invalid IDs, duplicate batch IDs, unknown source/status, and
  empty/zero/non-finite vectors return their typed errors with the specified
  precedence
- `Add` and `ReplaceAll` are all-or-nothing, deep-copy caller data, normalize
  private embeddings, and never mutate the caller's query or chunks
- `Add` upserts across calls without increasing `Len`; duplicate IDs within one
  batch reject the batch
- empty `Add`/`Remove` and removal of unknown valid IDs are successful no-ops
- concurrent `Add`, `ReplaceAll`, `Search`, `Remove`, and `Len` pass
  `go test -race`; cancellation returns no partial mutation/result
- dimension-mismatched chunks are skipped, and scores equal independently
  computed cosine similarity over the raw vectors
- `ListCurrent`, not `ListAll`, drives startup and excludes wrong-model,
  orphaned, stale, metadata-mismatched, and unknown-enum chunks according to
  the source-specific freshness rules
- one malformed/invalid persisted chunk produces `ready_with_warnings` while
  all valid chunks remain searchable; a database-wide failure produces
  `failed`, never a false ready-empty state
- the window renders `Loading knowledge index...` during initial background
  load; success releases the app, while failure offers `Retry` and
  `Continue without local search`
- continuing without search shows a persistent unavailable warning and local
  retrieval returns/wraps `ErrVectorStoreUnavailable`, not “no knowledge”; a
  direct `Search` before the first successful `ReplaceAll` returns the same
  sentinel
- status cannot be lost to an event-subscription race because the frontend
  subscribes and then performs an initial status query
- retry keeps an old valid snapshot searchable, blocks knowledge mutations,
  atomically publishes success, and preserves the old snapshot on failure
- `Review` identifies isolated chunks and gives source-appropriate recovery
  guidance without exposing raw internal errors
- changed-file re-import and Item deletion evict exactly the SQLite-deleted IDs
  after commit; rollback leaves the previous memory snapshot untouched
- re-importing a changed file leaves `Len` unchanged rather than growing and
  returns only the new content
- status/topic-only changes update SQLite and memory without an embedding call;
  changed searchable text creates new embeddings
- post-commit memory failure preserves the durable mutation, serves no
  deliberately restored stale content, and surfaces a retryable indexing
  warning rather than a false durable-write failure
- a functional 10,000 × 1536 test asserts correctness without wall-clock
  gating, and `BenchmarkSearch10Kx1536` records performance/allocation data
- the store and loader are usable from unit tests without a real database

## Explicit follow-up

This phase deliberately preserves case-sensitive Topic identity after
edge-trimming. The draft
[`13-canonical-topic-identity.md`](13-canonical-topic-identity.md) will define a
canonical topic key so `Go`, `go`, and equivalent variants become the same
topic while preserving a user-facing display name. That spec requires its own
grilling and must cover persistence migration, collisions, Explorer grouping,
filters, and cross-phase consumers; none of those semantics are hidden inside
this vector adapter.
