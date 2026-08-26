# Phase 2.8.2 — DeleteItem Orphan Chunk Risk

## Goal

Decide whether — and how — `DeleteItem` needs its own recovery path for a
post-commit `VectorStore.Remove` failure. This spec documents the problem
only; it intentionally stops short of picking a solution, the same way
`08-01-vectorstore-orphan-chunk-recovery.md` did before Option B was chosen
for `UpdateItem`.

## Why this is a separate spec

`08-01-vectorstore-orphan-chunk-recovery.md` covers the same root symptom —
a chunk evicted from SQLite but never evicted from the `VectorStore`,
because the post-commit `Remove` call failed — for `UpdateItem`'s
content-changed path. It settled on Option B: make `VectorStore.Add`
self-healing by evicting any existing chunk(s) for the same `ItemID` before
inserting a new one, so the next successful re-embed of that item cleans up
whatever `Remove` left behind.

`DeleteItem` (`internal/application/knowledge/delete.go`) hits the identical
failure at the identical point in its flow, but Option B does not reach it.
The difference matters enough to document separately rather than folding it
into 08-01's implementation.

## The problem

`DeleteItem`'s flow:

1. deletes the item's chunk row(s) from SQLite and the item row itself, in
   one transaction;
2. once that commits, calls `s.store.Remove(reconcileCtx, removedChunkIDs)`
   to evict the same chunk(s) from the in-memory `VectorStore`;
3. if that fails, returns `&IndexingWarning{Err: err}` — the delete itself
   is not rolled back, matching the existing failure policy of never
   undoing a durable write over a reconciliation failure.

If step 2 fails, the chunk stays behind in the `VectorStore`, carrying the
deleted item's `ItemID`, `Content`, and `Topic` — now searchable metadata
for an item that no longer exists in SQLite at all.

Option B's fix works by making `Add` clean up stale chunks the *next time*
something calls `Add` for that `ItemID` — typically the next successful
`indexKnowledgeItem` run, triggered by an edit or by the backfill sweep
(`ReindexKnowledgeItems`/`ListUnindexed`). `DeleteItem` removes the item row
itself, not just its chunk. There is no future event that reindexes a
deleted item — `ListUnindexed` only ever looks at items that still exist —
so no future `Add` call for that `ItemID` will ever happen. The orphaned
chunk has no path back to being cleaned up in-session; Option B's mechanism
never fires for it.

The orphan is still bounded by a restart, but for a different reason than
in 08-01: on restart, `ReplaceAll` rebuilds the entire `VectorStore`
snapshot from `ListCurrent`, which reads chunk rows from SQLite. Since the
chunk row was deleted from SQLite in the same transaction as the item, it
was never a candidate for the rebuilt snapshot to begin with — the orphan
simply isn't there to reload, the same way it wasn't there in 08-01's case.
Until that restart, `VectorStore.Search` can return a chunk whose `ItemID`
no longer resolves to any item — a dangling reference the UI has no defined
way to handle, unlike the two-versions-of-the-same-concept confusion 08-01
describes.

## Why 08-01's decision doesn't automatically extend here

It might seem like the same interface change should cover both. It doesn't,
because the two flows differ in exactly the property Option B depends on:

| | `UpdateItem` (08-01) | `DeleteItem` (this spec) |
|---|---|---|
| Item row after the failure | still exists | deleted |
| Future backfill visits it? | yes (`ListUnindexed`) | no |
| Future `Add` call for that `ItemID`? | yes, eventually | never |
| Option B cleans it up? | yes | no |

## Options considered (not evaluated in depth — for the next decision round)

- Extend `VectorStore.Remove` (or `DeleteItem` specifically) with the same
  kind of durable pending-removal bookkeeping 08-01's Option A described,
  scoped to deletes only.
- Give `VectorStore.Search` (or its caller) a defensive check that drops
  results whose `ItemID` doesn't resolve to a known item, independent of
  how the dangling chunk got there — closer to a symptom-level guard than a
  root fix.
- Accept the risk, matching 08-01's Option D reasoning: `Remove` failing at
  all is rare today (in-process store, no I/O), and the dangling result is
  bounded by the next restart.

## Decision

**Defensive guard in `Retrieve`**, decided during design discussion on
2026-08-25 — closest to the second option above ("give `Search` or its
caller a defensive check"), scoped specifically to `Retrieve`, the only
production caller of `VectorStore.Search` today.

Rationale: a partial answer is better than a hard failure. Before this
change, `Retrieve` resolved every surviving chunk's `ItemID` to its owning
`Item` via `s.items.GetByID`, and treated *any* resolution error —
including "item doesn't exist" — as fatal to the whole call. That means an
orphaned chunk scoring into top-K didn't just risk showing a stale result;
it could turn a successful RAG retrieval into an outright error for the
user's entire query, for as long as the process stayed up (this is a
desktop app that can run for days between restarts).

`Retrieve` now special-cases `errors.Is(err, knowledge.ErrItemNotFound)`:
that specific chunk is dropped from the survivor set and processing
continues with the rest. If dropping leaves zero survivors, `Retrieve`
returns the same empty result it already returns for "nothing relevant
found" — no new state. Any other `GetByID` error (e.g. the database itself
being unavailable) still aborts the call exactly as before — collapsing a
genuine infrastructure failure into "no local match" would hide a real
outage behind an experience indistinguishable from an empty index.

Extending `VectorStore.Remove`/`DeleteItem` with durable pending-removal
bookkeeping (this spec's first option, the `DeleteItem`-scoped analogue of
08-01's rejected Option A) was deliberately left out of this round. It
would actually evict the orphan from the `VectorStore` rather than just
hiding it from `Retrieve`, but it is more machinery — a new persistence
concept plus a retry sweep — than a failure this narrow (`Remove` failing
at all requires the in-process store itself to error, not a network call)
currently warrants. The guard above already removes the only reason this
was urgent: the hard failure. What remains is a purely internal drift (a
dead vector sitting in memory, silently skipped on every future `Search`,
until the next restart), which is now genuinely benign rather than a data
integrity risk.

To tell whether that drift is actually rare in practice — rather than
assuming it — every time the guard drops an orphaned chunk it logs
distinctly from the existing `IndexingWarning` log already emitted when
`DeleteItem`'s `Remove` call itself fails. That existing log only measures
how often `Remove` fails; it says nothing about whether the resulting
orphan ever actually surfaces in a search. If this new log turns out to
fire often, that is the signal to revisit durable bookkeeping.

### Implementation shape

- `internal/application/knowledge/retrieval.go` (`Service.Retrieve`): when
  resolving a survivor's `ItemID` via `s.items.GetByID` returns
  `knowledge.ErrItemNotFound`, log it and drop every chunk sharing that
  `ItemID` from the survivor set instead of returning an error. Any other
  error still aborts `Retrieve` unchanged.
- No changes to `DeleteItem`, `VectorStore.Add`/`Remove`, or any
  persistence layer — the fix is entirely inside `Retrieve`'s resolution
  loop.
- Tests: `internal/application/knowledge/retrieval_test.go` covers a sole
  orphaned survivor (yields the empty "no match" result), an orphaned
  chunk alongside a valid one (only the valid one survives), and a non-
  `ErrItemNotFound` resolution error (still propagates, unaffected).

### Known follow-ups (not part of this round)

- Any future caller of `VectorStore.Search` (e.g. 2.10's duplicate
  detection) will need to apply this same defensive pattern itself — this
  change does not generalize the guard beyond `Retrieve`.
- The study-session UI's error copy for a genuine infrastructure failure
  (as opposed to "nothing relevant found") was raised during design
  discussion as worth a friendlier message than a raw technical error, but
  that is a UI/copy concern outside this backend change's scope.

## References

- `08-01-vectorstore-orphan-chunk-recovery.md` — the parallel problem in
  `UpdateItem`, and the Option B decision this spec's table contrasts
  against.
- `internal/application/knowledge/delete.go` — `DeleteItem`.
- `internal/application/knowledge/retrieval.go` — `Service.Retrieve`, where
  the defensive guard now lives.
- `internal/domain/knowledge/vectorstore.go` — `VectorStore.Remove`/`Add`
  contracts.
- `internal/infrastructure/sqlite/chunk_repository.go` — `ListCurrent`,
  which rebuilds the `VectorStore` snapshot on restart from SQLite chunk
  rows only.
