# Phase 2.4 — Vector Search

## Goal

Pure-Go in-process vector store that retrieves the most relevant chunks by cosine similarity.

## Interface

```go
type VectorStore interface {
    Add(ctx context.Context, chunks []Chunk) error
    Search(ctx context.Context, query []float32, topK int, filters SearchFilters) ([]Chunk, error)
}

type SearchFilters struct {
    Topic  string
    Source string // athena | user_note | imported_doc
    Status string // draft | approved
}
```

## Tasks

- [ ] `internal/infrastructure/vectorstore/` — cosine similarity implementation
- [ ] Vectors loaded from `knowledge_chunks.embedding` (BLOB) on startup
- [ ] Top-K search with optional filters
- [ ] No external vector database dependency

## Acceptance Criteria

- `VectorStore.Search` returns at most K results, ordered by descending similarity
- Filters correctly restrict results by topic or source
- Adding 10,000 chunks and searching completes in under 500ms on a modern machine
- The store is usable from unit tests without a real database
