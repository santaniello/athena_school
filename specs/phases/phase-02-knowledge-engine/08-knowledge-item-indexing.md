# Phase 2.8 — Knowledge Item Indexing

## Goal

Knowledge Items are indexed in every lifecycle state. RAG retrieves only approved
items, while 2.10 can search drafts and deprecated items when detecting duplicate
knowledge.

## Why this is a separate spec

`specs/Athena.md` §9 makes "Athena Knowledge" a first-class source next to User Notes, and 2.4's `SearchFilters.Status` only makes sense if items reach the vector store. But the store does not exist until 2.4, while item creation and approval ship in 2.2/2.3 — so the hooks cannot live there, and items created before this spec need a backfill.

## Design decisions

These were worked out collaboratively before implementation; each row is the
consequence the decision rests on, not just the choice.

| Decision | Choice | Why |
|---|---|---|
| When does `UpdateItem` re-embed? | Only when Concept, Definition, Properties, or TradeOffs changed — a Topic-only (or RelatedConcepts-only) edit stays on the metadata-only path, since neither is part of the rendered chunk content. **Regardless of which path runs, the chunk's `ItemUpdatedAt` is always rewritten to match the Item's new `UpdatedAt`** — `UpdateItem` (and `TransitionTo`) restamp `UpdatedAt` on every write, content-changing or not, and `ListCurrent`'s staleness check is a blind equality comparison that doesn't know *why* the two drifted. Leaving `ItemUpdatedAt` behind on a metadata-only edit reproduces the exact "approved item vanishes from RAG after restart" bug this spec exists to fix, just triggered by a topic rename instead of a status transition. | Best of both worlds: no embedding cost for a rename, but the item never silently drops out of the index after a restart either way. |
| Does `SaveDrafts` keep trying to index the rest of a batch after one item's indexing fails? | No — stop attempting indexing for the remaining items in that call once the first one fails (all items still get **saved** regardless). Recovery for every skipped/failed item is uniformly the backfill flow below; the backfill query can't distinguish "tried and failed" from "never attempted," so there's no bookkeeping cost to skipping. | Avoids N sequential slow/failing embedding calls in one request when the OpenRouter key is missing or the machine is offline — `SaveDrafts` is an incidental, implicit save, not a user's explicit request to fully process a backlog. |
| Does `ReindexKnowledgeItems` (the "Index now" backfill run) stop at the first failure too? | No — it always attempts every unindexed item in the run, independent of earlier failures in the same run. | Unlike `SaveDrafts`, this is the user's explicit, consent-based "catch me up" action on a backlog that can be large; stopping at item 5 of 200 would leave 195 items that likely *would* index fine untouched, and force repeated manual clicks. Matches the existing `ImportFolder` precedent (one file's failure never aborts the rest of the batch). The unindexed-count alert simply reflects whatever remains after the run — it doesn't need to reach zero in one pass. |

## Indexing hook

`knowledge.Service` gains `chunks ChunkRepository` and `store VectorStore` (it already has `llm`), and:

- **`SaveDrafts`** calls `indexKnowledgeItem(ctx, item)` after each item's persistence — renders the item to a **single chunk** (concept + definition + properties + trade-offs), embeds it, saves with `Source = SourceAthena`, the item's current status, `ItemID`, `Topic`, `EmbeddingModel`, and `ItemUpdatedAt`, then calls `store.Add`. Items keep saving even after an indexing failure, but indexing itself is not reattempted for the rest of that batch (see Design decisions)
- **`Approve`** and **`Deprecate`** update the item plus an existing chunk's status and `ItemUpdatedAt` in one SQLite transaction, then replace its in-memory metadata without requesting another embedding; item content did not change. If the chunk is missing, the transition still commits and `indexKnowledgeItem` attempts the recoverable embedding afterwards. RAG filters `StatusApproved`, while duplicate detection deliberately searches all statuses
- **`UpdateItem`** persists the item. If Concept/Definition/Properties/TradeOffs changed, it deletes the old chunk in the same transaction as the item write, immediately evicts it from the in-memory store once that commits, then calls `indexKnowledgeItem` to re-embed — so retrieval and duplicate detection never use stale content, and a failed re-embed leaves the item with zero chunks (visible to backfill) rather than a stale one. If only Topic/RelatedConcepts changed, it stays on the cheaper metadata-only path used by Approve/Deprecate, still restamping `ItemUpdatedAt`
- **`DeleteItem`** calls `chunks.DeleteByItemID` + `store.Remove`, because deletion — unlike deprecation — removes the concept completely

## Failure policy

**An indexing failure must never roll back a successful Knowledge Item write.** The OpenRouter key may be missing or the machine offline.

The contract has to be explicit, because create/update/lifecycle operations must distinguish "item saved but not indexed" from "item write failed":

- The use case returns the **persisted item** plus an error wrapping `ErrIndexingFailed`
- The desktop binding checks `errors.Is(err, ErrIndexingFailed)`: it logs a warning and returns the item **successfully** to the frontend. Any other error is a real failure and propagates
- The item is already persisted before indexing is attempted, so the frontend's state is correct either way

**SQLite and the in-memory store can drift within a session** — a crash between `chunks.SaveAll` and `store.Add` leaves a persisted chunk that is not searchable. No compensation protocol is needed: the store is rebuilt from `knowledge_chunks` at every startup (2.4), so any drift self-heals on the next launch, and the backfill covers the case where nothing was persisted at all.

Stale content has the opposite policy: it is safer to return no local match than an
obsolete approved definition. After an item edit commits, remove its old in-memory
chunk before the embedding call. At startup, 2.4's `ListCurrent` excludes persisted
chunks whose `item_updated_at`, status, or embedding model no longer matches.

## Backfill

```sql
SELECT i...
  FROM knowledge_items i
  LEFT JOIN knowledge_chunks c ON c.item_id = i.id
 WHERE i.source = 'athena'
   AND (
        c.id IS NULL
     OR c.item_updated_at IS NULL
     OR c.item_updated_at <> i.updated_at
     OR c.status IS NULL
     OR c.status <> i.status
     OR c.embedding_model IS NULL
     OR c.embedding_model <> :current_embedding_model
   )
```

This detects missing **and stale** chunks. An update may commit successfully and then
fail while embedding; checking only `item_id NOT IN (...)` would leave the old content
searchable forever. `CountUnindexedItems` and `ReindexKnowledgeItems` receive the
current embedding model and use this same predicate.

**`i.source = 'athena'` excludes imported-note shadow Items (2.3) from this
query on purpose.** Their chunks always have `item_updated_at IS NULL` by
design — imported files use `ingested_files.mtime` for staleness, not
`item_updated_at` — so without this guard every imported note would show up as
permanently "unindexed" and get needlessly re-embedded on every "Index now"
run. Their `item_id` is populated (linking chunk to shadow Item) but their
freshness is governed entirely by 2.3's own re-import mechanism, independent of
this backfill.

The trigger is **not** silent-at-startup — that would spend the user's money without asking. On mount, the Knowledge Explorer calls `CountUnindexedKnowledgeItems`; if the count is `> 0` it renders an inline `Alert`:

```text
⚠ N knowledge items aren't indexed for search yet.        [ Index now ]
```

"Index now" runs `ReindexKnowledgeItems`, reusing the `ingest:progress` events from 2.3. This is discoverable, consent-based, and doubles as the recovery path for every indexing failure above — and unlike `SaveDrafts`, it never stops early: every unindexed item in the run gets attempted regardless of earlier failures in the same run (see Design decisions). The alert's count simply reflects whatever is still unindexed once the run ends; it does not have to reach zero in one pass.

## Tasks

Ordered so each step is independently testable (TDD red/green/refactor per AGENTS.md) and later steps build on earlier ones.

- [x] `internal/domain/knowledge/chunk.go` + `internal/infrastructure/sqlite/chunk_repository.go` — `ChunkRepository.UpdateMetadataByItemID` gains an `itemUpdatedAt time.Time` parameter and persists it; regenerate mocks
- [x] `internal/application/knowledge/indexing.go` — `indexKnowledgeItem(ctx, item) error`: render item → single chunk content, embed, delete any existing chunk(s) for the item + insert the new one in one SQLite transaction, then reconcile the VectorStore (`Remove` old IDs, `Add` new chunk)
- [x] `internal/application/knowledge/errors.go` — add the `ErrIndexingFailed` sentinel; make `IndexingWarning.Unwrap()` resolve to it so every call site can standardize on `errors.Is(err, ErrIndexingFailed)`
- [x] `internal/application/knowledge/extraction.go` — `saveCandidates` calls `indexKnowledgeItem` right after each item's `items.Save`; stop attempting indexing for the rest of the batch after the first indexing failure (saves are unaffected); propagate `ErrIndexingFailed`
- [x] `internal/interfaces/desktop/knowledge.go` — `SaveExtractedKnowledge`/`SaveAndApproveExtractedKnowledge` treat `ErrIndexingFailed` as a logged warning (same pattern already used by Approve/Deprecate/UpdateItem/DeleteItem), not a save failure
- [x] `internal/application/knowledge/approve.go` / `deprecate.go` — pass `item.UpdatedAt` into `UpdateMetadataByItemID`; when it returns zero updated chunks (chunk missing), call `indexKnowledgeItem` instead of `store.Add`
- [x] `internal/application/knowledge/update.go` — detect whether Concept/Definition/Properties/TradeOffs changed; unchanged → existing metadata-only path (now passing `item.UpdatedAt`); changed → delete the old chunk inside the item's own transaction, evict it from the VectorStore immediately after commit, then call `indexKnowledgeItem`
- [x] `internal/application/knowledge/delete.go` — confirm it already satisfies `errors.Is(err, ErrIndexingFailed)` after the `errors.go` change; no behavior change expected
- [x] `internal/domain/knowledge/repository.go` + `internal/infrastructure/sqlite/knowledge_repository.go` — the unindexed-items predicate as a shared SQL fragment, plus `CountUnindexed(ctx, embeddingModel)` and `ListUnindexed(ctx, embeddingModel)`
- [x] `internal/application/knowledge/backfill.go` — `CountUnindexedItems(ctx)`, `ReindexKnowledgeItems(ctx, onProgress)`: loops every unindexed item via `indexKnowledgeItem`, always continuing past individual failures, guarded by `IndexGuard.BeginMutation`/`EndMutation` for the whole run
- [x] `internal/interfaces/desktop/knowledge.go` — `CountUnindexedKnowledgeItems`, `ReindexKnowledgeItems`; the latter emits progress/done/error over the existing `ingest:progress`/`ingest:done`/`ingest:error` events with a reindex-shaped payload
- [x] `frontend/src/lib/knowledge.ts` — bindings + types for `countUnindexedKnowledgeItems`/`reindexKnowledgeItems`
- [x] `frontend/src/components/ingest-progress-dialog.tsx` — add a `reindex` kind with its own copy and a progress/summary shape suited to items (no "scanned"/"skipped" concepts)
- [x] `frontend/src/screens/KnowledgeExplorerScreen.tsx` — on-mount `CountUnindexedKnowledgeItems` check, the inline `Alert` + "Index now", re-checking the count once the dialog closes (a partial run can still leave it > 0)
- [x] `README.md` — the knowledge base as a feature, and the re-ingest-on-embedding-model-change consequence

## Acceptance Criteria

- Saving a draft creates exactly one chunk with `source = "athena"`, `status = "draft"`, and its `item_id`
- Approving or deprecating an item updates that chunk's status without an embedding call and never duplicates it
- Asking about that concept in `strict-notes` mode retrieves it, and the Sources strip identifies it as a knowledge item rather than a file
- Draft and deprecated chunks are excluded from RAG even though 2.10 can search them for duplicates
- A deprecated transition and its persisted chunk-status update are atomic, so deprecated knowledge cannot remain approved in RAG
- Approving an unindexed item retries indexing; failure preserves the approval and leaves it visible to backfill
- Deleting an item removes its chunk from both SQLite and the in-memory store, with no restart
- Editing an item's definition changes what search returns
- An indexing failure leaves the item persisted, wraps `ErrIndexingFailed`, and reaches the frontend as a success with a logged warning
- After an edit/indexing failure, the stale chunk is unreachable immediately and remains excluded after restart
- After an update/indexing failure, backfill counts and replaces the stale chunk rather than treating its mere presence as current
- Changing the embedding model makes every Knowledge Item eligible for re-indexing, even when vector dimensions are unchanged
- A chunk persisted without reaching the in-memory store becomes searchable after a restart, without manual intervention
- With items created before this spec, the explorer shows the backfill alert with the correct count; "Index now" processes them and the alert disappears
- `CountUnindexedItems` returns zero once every Knowledge Item has a chunk
