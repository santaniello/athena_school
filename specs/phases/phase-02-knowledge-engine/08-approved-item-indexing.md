# Phase 2.8 — Approved Item Indexing

## Goal

Approved Knowledge Items become retrievable by RAG alongside imported notes, so the curated base earned in 2.2/2.6/2.7 actually feeds back into study sessions.

## Why this is a separate spec

`specs/Athena.md` §9 makes "Athena Knowledge" a first-class source next to User Notes, and 2.4's `SearchFilters.Status` only makes sense if items reach the vector store. But the store does not exist until 2.4, while approval ships in 2.6 — so the hook cannot live in 2.6, and items approved before this spec need a backfill.

## Indexing hook

`Approve` was written in 2.6 as load → `TransitionTo` → `Update` precisely so this spec can extend it. `knowledge.Service` gains `chunks ChunkRepository` and `store VectorStore` (it already has `llm`), and:

- **`Approve`** appends `indexApprovedItem(ctx, item)` — renders the item to a **single chunk** (concept + definition + properties + trade-offs; items are short, so no chunking is needed), embeds it, saves with `Source = SourceAthena`, `Status = approved`, `ItemID = item.ID`, `Topic = item.Topic`, then `store.Add`
- **`Deprecate`** and **`DeleteItem`** call `chunks.DeleteByItemID` + `store.Remove`, so a retired item stops answering questions immediately rather than at the next restart
- **`UpdateItem`** on an approved item re-indexes it (delete then re-add), so an edited definition is what gets retrieved

## Failure policy

**An indexing failure must never roll back the approval.** The OpenRouter key may be missing or the machine offline.

The contract has to be explicit, because `Approve` returns `(KnowledgeItem, error)` and the binding must be able to tell "approved but not indexed" from "approval failed":

- `Approve` returns the **approved item** plus an error wrapping a new sentinel `ErrIndexingFailed`
- The desktop binding checks `errors.Is(err, ErrIndexingFailed)`: it logs a warning and returns the item **successfully** to the frontend. Any other error is a real failure and propagates
- The item is already persisted as approved before indexing is attempted, so the frontend's optimistic state is correct either way

**SQLite and the in-memory store can drift within a session** — a crash between `chunks.SaveAll` and `store.Add` leaves a persisted chunk that is not searchable. No compensation protocol is needed: the store is rebuilt from `knowledge_chunks` at every startup (2.4), so any drift self-heals on the next launch, and the backfill covers the case where nothing was persisted at all.

## Backfill

```sql
SELECT ... FROM knowledge_items
 WHERE status = 'approved'
   AND id NOT IN (SELECT item_id FROM knowledge_chunks WHERE item_id IS NOT NULL)
```

The trigger is **not** silent-at-startup — that would spend the user's money without asking. On mount, the Knowledge Explorer calls `CountUnindexedKnowledgeItems`; if the count is `> 0` it renders an inline `Alert`:

```text
⚠ N approved items aren't indexed for search yet.        [ Index now ]
```

"Index now" runs `ReindexApprovedItems`, reusing the `ingest:progress` events from 2.3. This is discoverable, consent-based, and doubles as the recovery path for every indexing failure above.

## Tasks

- [ ] `internal/application/knowledge/service.go` — add the `chunks` and `store` ports
- [ ] `internal/application/knowledge/indexing.go` — `indexApprovedItem`, item→chunk rendering
- [ ] `internal/application/knowledge/approve.go` / `deprecate.go` / `update.go` / `delete.go` — wire index, evict, and re-index
- [ ] `internal/application/knowledge/backfill.go` — `CountUnindexedItems(ctx)`, `ReindexApprovedItems(ctx, onProgress)`
- [ ] `internal/infrastructure/sqlite/knowledge_repository.go` — the unindexed-items query
- [ ] `internal/interfaces/desktop/knowledge.go` — `CountUnindexedKnowledgeItems`, `ReindexApprovedKnowledgeItems`
- [ ] `frontend/src/screens/KnowledgeExplorerScreen.tsx` — the backfill `Alert` + "Index now", reusing the ingest progress dialog
- [ ] `README.md` — the knowledge base as a feature, and the re-ingest-on-model-change consequence

## Acceptance Criteria

- Approving an item creates exactly one chunk with `source = "athena"`, `status = "approved"`, and its `item_id`
- Asking about that concept in `strict-notes` mode retrieves it, and the Sources strip identifies it as a knowledge item rather than a file
- Deprecating or deleting an item removes its chunk from both SQLite and the in-memory store, with no restart
- Editing an approved item's definition changes what retrieval returns
- An indexing failure leaves the item approved, wraps `ErrIndexingFailed`, and reaches the frontend as a success with a logged warning — not as a failed approval
- A chunk persisted without reaching the in-memory store becomes searchable after a restart, without manual intervention
- With items approved before this spec, the explorer shows the backfill alert with the correct count; "Index now" processes them and the alert disappears
- `CountUnindexedItems` returns zero once every approved item has a chunk
