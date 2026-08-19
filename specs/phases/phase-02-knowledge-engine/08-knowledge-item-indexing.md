# Phase 2.8 — Knowledge Item Indexing

## Goal

Knowledge Items are indexed in every lifecycle state. RAG retrieves only approved
items, while 2.10 can search drafts and deprecated items when detecting duplicate
knowledge.

## Why this is a separate spec

`specs/Athena.md` §9 makes "Athena Knowledge" a first-class source next to User Notes, and 2.4's `SearchFilters.Status` only makes sense if items reach the vector store. But the store does not exist until 2.4, while item creation and approval ship in 2.2/2.3 — so the hooks cannot live there, and items created before this spec need a backfill.

## Indexing hook

`knowledge.Service` gains `chunks ChunkRepository` and `store VectorStore` (it already has `llm`), and:

- **`SaveDrafts`** calls `indexKnowledgeItem(ctx, item)` after persistence — renders the item to a **single chunk** (concept + definition + properties + trade-offs), embeds it, saves with `Source = SourceAthena`, the item's current status, `ItemID`, `Topic`, `EmbeddingModel`, and `ItemUpdatedAt`, then calls `store.Add`
- **`Approve`** and **`Deprecate`** update the item plus an existing chunk's status and `ItemUpdatedAt` in one SQLite transaction, then replace its in-memory metadata without requesting another embedding; item content did not change. If the chunk is missing, the transition still commits and `indexKnowledgeItem` attempts the recoverable embedding afterwards. RAG filters `StatusApproved`, while duplicate detection deliberately searches all statuses
- **`UpdateItem`** persists the item, immediately evicts its old in-memory chunk, then re-indexes it so retrieval and duplicate detection never use stale content. If embedding fails, the item remains saved but absent from search until backfill
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

"Index now" runs `ReindexKnowledgeItems`, reusing the `ingest:progress` events from 2.3. This is discoverable, consent-based, and doubles as the recovery path for every indexing failure above.

## Tasks

- [ ] `internal/application/knowledge/service.go` — add the `chunks` and `store` ports
- [ ] `internal/application/knowledge/indexing.go` — `indexKnowledgeItem`, item→chunk rendering
- [ ] `internal/domain/knowledge/chunk.go` — add the status/`ItemUpdatedAt` update needed by lifecycle transitions
- [ ] `internal/application/knowledge/approve.go` / `deprecate.go` / `update.go` / `delete.go` — wire metadata update, re-index, and eviction
- [ ] `internal/application/knowledge/extraction.go` — index newly saved drafts without turning an indexing warning into a failed save
- [ ] `internal/application/knowledge/backfill.go` — `CountUnindexedItems(ctx)`, `ReindexKnowledgeItems(ctx, onProgress)`
- [ ] `internal/infrastructure/sqlite/knowledge_repository.go` — the unindexed-items query
- [ ] `internal/interfaces/desktop/knowledge.go` — `CountUnindexedKnowledgeItems`, `ReindexKnowledgeItems`
- [ ] `frontend/src/screens/KnowledgeExplorerScreen.tsx` — the backfill `Alert` + "Index now", reusing the ingest progress dialog
- [ ] `README.md` — the knowledge base as a feature, and the re-ingest-on-model-change consequence

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
