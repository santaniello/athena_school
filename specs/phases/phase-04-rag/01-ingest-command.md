# Spec: Ingest Command

## Goal

Allow the user to index their local notes (Markdown files) so Athena can use them as a personal knowledge base. This is the entry point to the RAG (Retrieval-Augmented Generation) pipeline.

## User Story

> As an Obsidian user, I want to run `athena ingest ./notes` so that my existing notes become the knowledge source for all Athena sessions.

## Acceptance Criteria

- [ ] `athena ingest <path>` recursively finds all `.md` files under the path
- [ ] Each file is chunked, embedded, and stored in a local vector index
- [ ] Progress is shown during ingestion (e.g., `[12/47] indexing caching.md`)
- [ ] Re-running ingest on the same path is idempotent (only new/changed files are processed)
- [ ] `athena ingest --status` shows how many notes are indexed
- [ ] `athena ingest --clear` removes all indexed notes (with confirmation prompt)

## CLI Usage

```bash
athena ingest ./notes
athena ingest ~/obsidian/system-design
athena ingest --status
athena ingest --clear
```

## Pipeline

```
.md files
    ↓
Chunking (split by heading or ~500 tokens)
    ↓
Embedding (via LLM provider)
    ↓
Vector store (local file-based)
```

## Directory Structure

```
internal/
└── rag/
    ├── ingest/
    │   ├── scanner.go       # walk filesystem, find .md files
    │   ├── chunker.go       # split document into chunks
    │   └── pipeline.go      # orchestrate scan → chunk → embed → store
    ├── embed/
    │   └── embedder.go      # calls LLMProvider.Embeddings()
    └── store/
        ├── store.go         # VectorStore interface
        └── jsonstore/
            └── jsonstore.go # file-based store (MVP)
cmd/athena/
└── cmd_ingest.go
```

## Chunking Strategy

- Split on `##` or `###` headings first
- If a chunk exceeds 500 tokens, split on paragraph boundaries
- Each chunk retains the file path and heading as metadata

## Chunk Metadata

```go
type Chunk struct {
    ID        string
    FilePath  string
    Heading   string
    Content   string
    Embedding []float32
    Hash      string   // SHA256 of Content, for change detection
}
```

## Storage Format

File: `~/.config/athena/index.json`

```json
[
  {
    "id": "uuid",
    "file_path": "/notes/caching.md",
    "heading": "Cache Invalidation",
    "content": "...",
    "embedding": [0.12, -0.34, ...],
    "hash": "sha256..."
  }
]
```

## Idempotency

- On re-run, compute `SHA256(content)` for each chunk
- Skip chunks whose hash already exists in the store
- Remove stale entries for files that no longer exist on disk

## LLM Provider Extension

The `LLMProvider` interface must be extended to support embeddings:

```go
type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Embeddings(ctx context.Context, inputs []string) ([][]float32, error)
}
```

Ollama supports embeddings via `/api/embeddings`.

## Terminal Output Format

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Ingesting notes from ./notes
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[1/12]  caching.md            ✓
[2/12]  load-balancing.md     ✓
[3/12]  sharding.md           ✓ (3 chunks)
...

✅ Ingestion complete
   12 files · 34 chunks · stored in ~/.config/athena/index.json
```

## Done When

```bash
$ athena ingest ./notes
# → processes files, prints progress, stores index

$ athena ingest --status
# 34 chunks indexed from 12 files
# Last ingested: 2026-04-03 10:30
```
