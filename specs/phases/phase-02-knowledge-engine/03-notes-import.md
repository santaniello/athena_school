# Phase 2.3 — Notes Import (Markdown)

## Goal

User imports a folder of personal notes; the pipeline parses, chunks, embeds, and stores them for RAG retrieval.

## Pipeline

```text
Markdown files
    ↓
Parser (goldmark)
    ↓
Chunking (by heading or fixed size ~500 tokens)
    ↓
Metadata (source, file_path, heading, created_at)
    ↓
Embeddings (OpenRouter embedding model)
    ↓
Local vector store
```

## Supported Formats

- `.md` (primary)
- `.txt`

## Schema

```sql
CREATE TABLE knowledge_chunks (
    id         TEXT PRIMARY KEY,
    source     TEXT,
    file_path  TEXT,
    heading    TEXT,
    content    TEXT,
    embedding  BLOB, -- serialized float32 vector
    created_at DATETIME
);
```

## Tasks

- [ ] `internal/application/ingest/` — ingest pipeline use case
- [ ] UI: "Import notes" button + folder picker (Wails dialog)
- [ ] Progress indicator during import (file count, chunk count)
- [ ] Deduplication: skip files whose path + mtime haven't changed
- [ ] Error handling: log bad files and continue; report summary at the end

## Acceptance Criteria

- User picks a folder; all `.md` and `.txt` files are ingested
- Each file is split into chunks; each chunk has an embedding stored in `knowledge_chunks`
- Re-importing the same folder is idempotent (no duplicates)
- A folder with 100 markdown files completes without crashing
