# Spec: Semantic Search & RAG Retrieval

## Goal

When a session uses `--source notes` or `--source strict-notes`, retrieve the most relevant chunks from the vector index and inject them as context into the LLM prompt.

## User Story

> As a developer, I want Athena to automatically find the relevant parts of my notes when I study a topic, so that feedback and questions reflect what I actually wrote.

## Acceptance Criteria

- [ ] Given a topic query, the system returns the top-K most relevant note chunks
- [ ] Relevance is determined by cosine similarity between query embedding and chunk embeddings
- [ ] Retrieved chunks are formatted and injected into the session prompt (as defined in Source Modes spec)
- [ ] If no relevant chunks are found (similarity below threshold), a warning is shown: `⚠️ No notes found for this topic`
- [ ] Number of retrieved chunks is configurable (default: 5)
- [ ] Retrieval works without a remote server — pure local computation

## Directory Structure

```
internal/
└── rag/
    └── retrieval/
        ├── retriever.go       # Retriever interface + TopK()
        ├── cosine.go          # CosineSimilarity(a, b []float32) float32
        └── retriever_test.go
```

## Retriever Interface

```go
type Retriever interface {
    TopK(ctx context.Context, query string, k int) ([]store.Chunk, error)
}
```

## TopK Algorithm

```
1. Embed the query string using LLMProvider.Embeddings()
2. Load all chunks from the store
3. Compute cosine similarity between query embedding and each chunk
4. Sort by similarity descending
5. Return top K chunks where similarity >= threshold (default: 0.7)
```

## Cosine Similarity

```go
func CosineSimilarity(a, b []float32) float32 {
    // dot(a,b) / (norm(a) * norm(b))
}
```

## Context Injection

The retrieved chunks are formatted before being injected into the system prompt:

```
--- USER NOTES ---
[caching.md › Cache Invalidation]
When a cache entry becomes stale, there are three main strategies:
TTL-based expiry, event-driven invalidation, and write-through...

[caching.md › LRU Eviction]
The Least Recently Used policy evicts the entry that was accessed...
-----------------
```

## Source Mode Integration

In `source/injector.go`:

```go
func BuildContext(mode SourceMode, query string, retriever retrieval.Retriever) (string, error) {
    if mode == Web {
        return "", nil
    }
    chunks, err := retriever.TopK(ctx, query, 5)
    // format chunks → inject into prompt template
}
```

## Implementation Notes

- Cosine similarity is computed in-process — no external vector DB needed for MVP
- For large indexes (> 10k chunks), consider adding an inverted keyword index as a pre-filter
- Similarity threshold is configurable in config: `athena config set rag.threshold 0.7`
- If `LLMProvider.Embeddings()` is not available (future providers), fall back to keyword search

## Done When

```bash
$ athena ingest ./notes
$ athena study system-design caching --source notes
# → Athena retrieves relevant caching chunks and uses them in the explanation + evaluation
```
