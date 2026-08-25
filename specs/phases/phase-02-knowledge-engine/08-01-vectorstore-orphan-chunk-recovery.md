# Phase 2.8.1 — VectorStore Orphan Chunk Recovery

## Goal

Decide how `UpdateItem`'s re-index path recovers when `VectorStore.Remove`
itself fails, so a lost eviction can never leave two versions of the same
concept searchable at once. This spec documents the problem and the
candidate solutions; it intentionally stops short of picking one — that
decision needs to be made before implementation starts.

## Why this is a separate spec

2.8 (`08-knowledge-item-indexing.md`) already covers the ordinary indexing
failure modes — a missing OpenRouter key, an offline machine, a crash between
`chunks.SaveAll` and `store.Add` — and gives each one a recovery path: the
item stays persisted, the backfill query (`ListUnindexed`) picks it back up,
and a later successful `indexKnowledgeItem` call makes it searchable again.
That backfill query works by testing whether a *current* chunk exists for the
item; it says nothing about whether a *stale* one still lingers in the
`VectorStore`. The specific failure below falls through that gap: the item
comes back onto the backfill list correctly, but nothing ever asks the
`VectorStore` to forget the chunk it failed to evict, and `VectorStore.Add`
has no way to find it on its own.

## The problem

`UpdateItem`'s content-changed path (`internal/application/knowledge/update.go`):

1. deletes the item's old chunk row from SQLite, in the same transaction as
   the item write;
2. once that commits, calls `s.store.Remove(reconcileCtx, removedChunkIDs)`
   to evict the same chunk from the in-memory `VectorStore`;
3. only if that succeeds, calls `indexKnowledgeItem` to embed the new content
   and add the new chunk.

If step 2 fails, `UpdateItem` returns immediately, wrapping
`ErrIndexingFailed` — matching the existing failure policy of never rolling
back a durable write. `TestUpdateItem_returnsErrIndexingFailed_whenEvictingTheStaleChunkFails`
documents exactly this today. But `removedChunkIDs` is a local variable: once
the call returns, that ID is gone. Nothing persists it, and nothing retries
the eviction later.

The item now has zero chunks in SQLite, so `ListUnindexed` correctly flags it
as unindexed and a later "Index now" run (or the next successful edit) will
call `indexKnowledgeItem` again. That call embeds the new content into a
**new** `Chunk` with a **new** `uuid.NewString()` ID
(`internal/application/knowledge/indexing.go`), and adds it via
`VectorStore.Add`. Per `internal/domain/knowledge/vectorstore.go`:

> `Add` validates and normalizes the batch, then upserts it by `Chunk.ID`: an
> existing ID is replaced in place, a new one is appended.

`Add` only knows how to replace by `Chunk.ID`. It has no notion of `ItemID`.
Since the new chunk has a different ID than the one `Remove` failed to evict,
the old, orphaned vector is never touched — it stays in the `VectorStore`
indefinitely, alongside the new one, both carrying the same `ItemID` but
different `Content`. `VectorStore.Search` can then return the stale and the
current definition for the same concept side by side, and nothing short of a
process restart (which rebuilds the store from `ListCurrent`, itself immune
to this because it reads SQLite, not the `VectorStore`) clears it.

This is narrow — it only triggers when `Remove` itself fails, which today
only happens if the in-process `VectorStore` returns an error, not a network
call — but the failure window exists and the resulting drift is silent and
permanent within the running session.

## Options considered

### Option A — Persist the pending removal

Add durable bookkeeping (e.g. a `pending_chunk_removals` table, or reusing an
existing table with a status column) that records `removedChunkIDs` *before*
calling `store.Remove`, and clears the record only once `Remove` confirms
success. A sweep — run at startup, or folded into `ReindexKnowledgeItems` —
retries every pending removal against the `VectorStore` until it succeeds.

- Pro: precise — only ever retries IDs that are actually known to be stale,
  no risk of evicting a chunk that never needed it.
- Con: a new persistence concept and a new sweep/retry path; more surface
  area than the other options, and another thing that itself needs a
  recovery story if the sweep is skipped.

### Option B — Make `VectorStore.Add` self-healing by `ItemID`

Change `VectorStore.Add`'s contract so it evicts any existing chunk(s) for
the same `ItemID` before inserting the new one, instead of upserting by
`Chunk.ID` alone. A later successful `indexKnowledgeItem` call then always
converges to exactly one vector per `ItemID`, regardless of whether an
earlier `Remove` was ever attempted or lost.

- Pro: small, local change (`internal/infrastructure/vectorstore` +
  the `VectorStore` interface doc); no new persistence, no sweep; also
  incidentally hardens against any other future path that adds a chunk
  without first removing the old one.
- Con: changes a documented interface contract — every existing `Add`
  caller and test that asserts upsert-by-`Chunk.ID` semantics needs
  re-auditing; `Add` gains an implicit linear scan by `ItemID` per call
  (bounded by however many chunks share an `ItemID`, which today is always
  0 or 1 outside this failure).

### Option C — Atomic replace by `ItemID` (`Remove` + `Add` as one call)

Introduce a single `VectorStore.ReplaceByItemID(ctx, itemID, chunk)` (or
similar) that evicts every chunk for `itemID` and inserts the new one as one
operation, and have `indexKnowledgeItem` call it instead of separate
`Remove`/`Add`. This removes the two-step window entirely rather than
recovering from it after the fact.

- Pro: closest to how the bug is actually described — the eviction and the
  insertion were always meant to be one logical step; collapsing them
  removes the partial-failure state instead of papering over it.
- Con: largest change of the three — a new `VectorStore` method, every call
  site of the old `Remove`+`Add` pair to migrate, and a decision about what
  it means for the operation to partially fail (does it still return the
  evicted IDs on error, the way `Remove` and `DeleteByItemID` do today?).

### Option D — Accept the risk, do nothing for now

`Remove` failing at all is rare in the current in-process `VectorStore`
implementation (no network call, no I/O), and the resulting drift is a
search-quality issue, not data loss or corruption. Leave
`TestUpdateItem_returnsErrIndexingFailed_whenEvictingTheStaleChunkFails` as
the documented, accepted behavior for now and revisit if a future
`VectorStore` implementation (e.g. an external service) makes `Remove`
failures common enough to matter.

## Decision

**Option B**, decided during design discussion on 2026-08-25.

Rationale: the failure that opens this window is rare and in-process (no
network I/O), and the resulting drift is already bounded by a restart
(`ReplaceAll` rebuilds the snapshot from SQLite, which never held the
orphan). Option A's durable pending-removal bookkeeping is disproportionate
to a failure this narrow — it adds a permanent mechanism, plus its own
retry/recovery story, for a problem that already self-heals. Option C fixes
the root cause but at the cost of migrating every existing `Remove`+`Add`
call site (`approve.go`, `deprecate.go`, `delete.go`, `indexing.go`,
`update.go`) to a new atomic method and defining its partial-failure
semantics — more surface than the bug warrants. Option D alone was rejected
because this is a desktop app that can stay open for days between restarts;
in that window, a search could return two conflicting definitions of the
same concept with no signal to the user, and Option B is cheap enough that
accepting that risk isn't worth it.

Option B was chosen specifically because it converges automatically the
next time anything calls `Add` for the same `ItemID` — typically the next
successful `indexKnowledgeItem` run, whether triggered by another edit or
by the backfill sweep. It does **not** cover `DeleteItem`'s parallel
failure, where the item row itself (not just the chunk) is gone and no
future `Add` for that `ItemID` will ever happen — see
`08-02-deleteitem-orphan-chunk-risk.md`, split out separately rather than
folded into this implementation.

A UI-facing warning for this specific failure (beyond the existing
`log.Printf` in `logIndexingFailure`) was considered and deliberately left
out: Option B already removes the user-visible symptom (the duplicate
search result) automatically, so the warning would only announce a state
that self-corrects before anyone acts on it.

### Implementation shape

- `internal/infrastructure/vectorstore/store.go` (`Store.Add`): before
  upserting the incoming batch, evict any existing chunk that shares an
  incoming chunk's `ItemID` but has a different `Chunk.ID`. Plain
  upsert-by-`Chunk.ID` behavior (existing ID replaced in place) is
  unchanged.
- `internal/domain/knowledge/vectorstore.go`: update `VectorStore.Add`'s
  doc comment to describe the new by-`ItemID` eviction, not just
  upsert-by-`Chunk.ID`.
- No application-layer call site changes — `approve.go`, `deprecate.go`,
  `indexing.go`, `update.go` keep calling `Add` exactly as they do today.
- Tests: new `vectorstore` package coverage for the by-`ItemID` eviction;
  extend `TestUpdateItem_returnsErrIndexingFailed_whenEvictingTheStaleChunkFails`
  (or add a companion test) to prove a subsequent `indexKnowledgeItem` run
  leaves the store with exactly one chunk for the item, not two; audit
  existing `Add` tests that assert upsert-by-`Chunk.ID` semantics for any
  that would break under the new eviction.

## References

- Raised by CodeRabbit on PR #51 (`internal/application/knowledge/update_test.go`),
  against `08-knowledge-item-indexing.md`.
- `internal/application/knowledge/update.go` — `UpdateItem`'s content-changed
  path.
- `internal/application/knowledge/indexing.go` — `indexKnowledgeItem`.
- `internal/domain/knowledge/vectorstore.go` — `VectorStore.Add`/`Remove`
  contracts.
