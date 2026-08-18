# Phase 2.4 — Vector Search

## Goal

Pure-Go in-process vector store that retrieves the most relevant chunks by cosine similarity, with no external vector database.

## Interface

```go
type SearchFilters struct{ Topic, Source, Status string } // "" = no constraint

type ScoredChunk struct {
    Chunk Chunk
    Score float32
}

type VectorStore interface {
    Add(ctx context.Context, chunks []Chunk) error
    Search(ctx context.Context, query []float32, topK int, filters SearchFilters) ([]ScoredChunk, error)
    Remove(ctx context.Context, ids []string) error
    Len() int
}
```

Three deliberate departures from the original sketch, each earning its place:

- **`[]ScoredChunk` instead of `[]Chunk`** — 2.5's sufficiency threshold and its sources footer are both defined on the similarity score; hiding it would force recomputation.
- **`Remove`** — without it, a deleted or deprecated item keeps answering questions until the app restarts. That is a correctness bug, not a nice-to-have.
- **`Len`** — lets 2.5 skip the paid embedding call entirely when the knowledge base is empty, which is the common case for every existing user on first upgrade.

## Implementation

- Memory shape: `struct { mu sync.RWMutex; chunks []knowledge.Chunk }` — a flat, contiguous, cache-friendly slice. No per-topic index; brute force is fast enough by two orders of magnitude.
- **`Add` normalizes each stored vector to unit length in memory, and `Search` normalizes the query once before scoring.** Persistence keeps the provider's raw output, so reloading always re-normalizes and nothing drifts. Cosine reduces to a plain dot product **only when both sides are unit length** — normalizing just the stored side would scale every score by the query's norm, which is not cosine. A zero-norm vector on either side is rejected rather than divided by.
- **`Search` pre-filters, then scores**: skip a chunk on any set-and-mismatched filter before touching its vector, so filters make queries cheaper rather than more expensive. Skip dimension mismatches defensively, so a mixed-dimension database degrades instead of returning garbage. Then `sort.SliceStable` descending and truncate to `topK`. `topK <= 0` → `ErrInvalidTopK`.
- **Loaded at startup, synchronously, in `main.go`** right after `sqlite.Open`: `chunks.ListAll` → `store.Add`. The app already shows a splash screen, and realistic corpora are 1–3k chunks (6–18 MB).
- The ingest use case calls `store.Add` after persisting, so newly imported notes are searchable without a restart. That wiring is why the 2.3 → 2.4 order produces **no throwaway code**: 2.3 ships persistence only, 2.4 layers memory on top.
- **Re-importing a changed file must `store.Remove` its old chunk IDs before adding the new ones.** 2.3's `chunks.DeleteByFilePath` only clears SQLite; without the matching in-memory eviction the previous version of the file keeps answering queries until the next restart, and `Len` grows on every import. This is the same class of bug `Remove` exists for in 2.8 — it applies to re-ingestion just as much as to item deletion.

Budget check: 10k × 1536 float32 multiply-adds ≈ 15.4 MFLOP ≈ 5–15 ms single-threaded in Go, plus a 10k-element sort. Comfortably inside the 500 ms criterion.

## ADR-004

This spec ships with `specs/decisions/ADR-004-local-vector-store.md`:

1. **Decision** — in-process brute-force cosine over a flat slice, pure Go, loaded from `knowledge_chunks` at startup
2. **Rejected** — `sqlite-vec` (requires CGO, forbidden by `AGENTS.md`); `philippgille/chromem-go` (pure Go and genuinely viable, but it owns its own persistence, duplicating `knowledge_chunks`, for what amounts to ~150 lines of dot product and a sort); any hosted vector DB (violates local-first)
3. **Embedding model** — `openai/text-embedding-3-small` (1536d) inherited from `llm.EmbeddingModel`; float32 LE BLOB encoding; **consequence: changing the model requires a full re-ingest**
4. Normalization at insert; cosine → dot product
5. Memory profile, and the escape hatch (async loading behind a ready flag) if a corpus ever makes startup loading painful
6. **Amends ADR-002's scope**: `internal/infrastructure/vectorstore` joins `make mutation-go`. The rest of `internal/infrastructure` stays out — the SQLite repositories are thin mapping and the OpenRouter client is HTTP plumbing, so including them would cost CI time and mutant noise for little signal

## Tasks

- [ ] `internal/domain/knowledge/vectorstore.go` — `SearchFilters`, `ScoredChunk`, `VectorStore`, `ErrInvalidTopK`
- [ ] `internal/infrastructure/vectorstore/store.go` — normalization on `Add`, pre-filter-then-score `Search`, `Remove`, `Len`
- [ ] `main.go` — construct the store and load `chunks.ListAll` into it after `sqlite.Open`
- [ ] `internal/application/ingest/import_folder.go` — call `store.Add` after persisting each file's chunks
- [ ] `specs/decisions/ADR-004-local-vector-store.md`
- [ ] `Makefile` — add `internal/infrastructure/vectorstore` to the `mutation-go` loop

## Acceptance Criteria

- `Search` returns at most `topK` results, ordered by descending similarity, with the score exposed
- Filters correctly restrict results by topic, source, and status; an empty filter field imposes no constraint
- `topK <= 0` returns `ErrInvalidTopK`
- A chunk whose embedding dimension differs from the query is skipped, not scored
- `Remove` makes a chunk unreachable from subsequent searches without a restart
- `Len` reports the number of loaded chunks
- Scores match cosine similarity computed independently on the raw (un-normalized) vectors — the query is normalized, not just the stored side
- Re-importing a changed file leaves `Len` unchanged rather than growing, and searches return only the new version's chunks
- Adding 10,000 chunks and searching completes in under 500 ms on a modern machine
- The store is usable from unit tests without a real database
