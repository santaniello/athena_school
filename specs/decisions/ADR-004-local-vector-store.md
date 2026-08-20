# ADR-004 — Local Vector Store

**Status:** Accepted
**Date:** 2026-08-19

---

## Context

Phase 2.4 ([04-vector-search.md](../phases/phase-02-knowledge-engine/04-vector-search.md)) needs to retrieve the most relevant current knowledge chunks by cosine similarity over their embeddings, so Phase 2.5's RAG pipeline has something to inject into the study prompt. `AGENTS.md` forbids CGO (the SQLite driver, `modernc.org/sqlite`, is pure Go specifically so `wails build` never needs a C toolchain on the target machine), and the product is local-first: no server, no hosted database, one SQLite file at `~/.athena/athena.db` per user.

`knowledge_chunks` (added in Phase 2.3) already persists `id, source, topic, status, item_id, file_path, heading, content, embedding, embedding_model, item_updated_at, created_at`, with `embedding` as a tightly-packed little-endian `float32` BLOB. Realistic corpora are in the 1–3k chunk range; the acceptance criterion is a functional 10,000×1536 test with a documented-machine target under 500 ms, not a hard CI gate (shared runners have unpredictable clock speed).

---

## Decision

1. **In-process, brute-force cosine similarity over a flat `[]knowledge.Chunk` slice, pure Go, with SQLite as the durable source of truth.** `internal/infrastructure/vectorstore.Store` holds no persistence of its own — it is rebuilt from SQLite via `ChunkRepository.ListCurrent` on every application start and kept coherent afterward by explicit `Add`/`ReplaceAll`/`Remove` calls from the use cases that already mutate `knowledge_chunks` (import, approve, deprecate, update, delete). No per-topic or approximate-nearest-neighbor index: 10,000×1536 float32 multiply-adds is ~15.4 MFLOP, comfortably inside budget (measured `BenchmarkSearch10Kx1536`: ~11 ms/op on a 2018 Core i7-8550U — a full order of magnitude under the 500 ms target on hardware far from "modern").

2. **Rejected alternatives:**
   - **`sqlite-vec`** — a real SQLite extension for vector search, but it ships as a C extension loaded via `cgo`-dependent bindings, violating the no-CGO rule outright.
   - **`philippgille/chromem-go`** — pure Go and genuinely viable, but it owns its own on-disk persistence (a second embedded store), duplicating `knowledge_chunks` for what amounts to ~150 lines of dot product and a sort. It would also fight this spec's "SQLite is the single source of truth, memory is a derived cache" design rather than support it.
   - **Hosted vector databases** (Pinecone, Qdrant Cloud, etc.) — violate local-first outright; the product has no server component and no user account tied to a remote index.

3. **Embedding model and persistence format are inherited from Phase 2.3, not redecided here**: `openai/text-embedding-3-small` (1536 dimensions) via `llm.EmbeddingModel`, raw `float32` little-endian BLOB. Changing the embedding model is a breaking change to what's already indexed — `ListCurrent` excludes chunks from a different model rather than guessing, and re-ingestion/reindexing (this phase's re-import flow, Phase 2.8's backfill for Athena items) is the only way to migrate.

4. **Store semantics**: `Add`/`ReplaceAll` normalize every embedding to unit length after deep-copying it (the provider's raw vector, as validated, is never mutated — SQLite keeps the un-normalized original), validate the complete batch before mutating anything (`knowledge.ValidateChunk`/`ValidateVector`, shared with the SQLite loader's own defense-in-depth re-validation), reject a batch with a duplicate ID, and publish atomically under a single `sync.RWMutex`. `Add` upserts by ID across calls without growing `Len`; `ReplaceAll` swaps the entire snapshot, including publishing a real (not skipped) empty snapshot. `Search` normalizes its query once — cosine similarity is a plain dot product only when *both* operands are unit vectors — orders by score descending then `Chunk.ID` ascending for deterministic ties, and returns deep copies so a caller can never mutate the store through a search result.

5. **Background loading with an explicit lifecycle, not a synchronous pre-`wails.Run` load.** The coordinator (`internal/application/knowledge.IndexLoader`) exposes `loading` / `ready` / `ready_with_warnings` / `failed`, starts from Wails' `OnDomReady` so the window renders immediately, and isolates one corrupt/stale row (`ChunkLoadIssue`) instead of failing the entire load. A retry keeps the previous snapshot searchable until a new one is atomically published, and never on a failed retry. A never-loaded or failed store returns `ErrVectorStoreUnavailable` from `Search`, distinct from "loaded and genuinely empty" — search code must never translate the former into "no knowledge found."

6. **Performance is a benchmark-tracked regression gate, not a hard CI assertion.** `BenchmarkSearch10Kx1536` records time/allocations for comparison across changes; the 10k functional correctness test asserts count/order/score correctness with no wall-clock assertion, since shared CI runners are not a stable timing environment.

7. **Amends [ADR-002](ADR-002-mutation-testing.md)'s scope**: `internal/infrastructure/vectorstore` joins the `mutation-go` loop as a third, explicitly-scoped entry (`gremlins unleash ./internal/infrastructure/vectorstore`) — it is real, hand-written business logic (validation precedence, atomic swap, cosine correctness) that deserves the same mutation-testing gate as `domain`/`application`, unlike the thin SQLite mapping/HTTP adapters the rest of `internal/infrastructure` still excludes.

---

## Consequences

**Positive:**
- Zero new runtime dependencies, zero CGO, zero second on-disk store to keep consistent with `knowledge_chunks` — SQLite stays the only durable state.
- Search correctness and performance are both regression-tested (`go test -race`, the 10k functional case, `BenchmarkSearch10Kx1536`), and `internal/domain`'s `ValidateChunk`/`ValidateVector` give the store and the SQLite loader one shared definition of "valid chunk" instead of two.
- The explicit lifecycle (loading/ready/ready_with_warnings/failed) makes a corrupt row or a mid-retry state observable and recoverable from the UI, rather than a silent gap in RAG results.

**Negative:**
- Every chunk's embedding is held in memory at once (10k×1536×4 bytes ≈ 60 MB for the stress-test corpus, well under budget for a desktop app; a real user's 1–3k-chunk corpus is a few MB). This ADR's escape hatch, if a corpus ever makes that painful, is the same one considered and dropped for this phase: an approximate-nearest-neighbor index or a paged/quantized representation — deliberately not built now, since nothing in the product today produces a corpus anywhere near that size.
- Brute-force search is O(n) per query; acceptable at the sizes this product expects, but a future phase targeting tens of thousands of chunks per user would need to revisit this ADR.
