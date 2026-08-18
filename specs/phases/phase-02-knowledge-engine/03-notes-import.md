# Phase 2.3 — Notes Import (Markdown)

## Goal

User imports a folder of personal notes; the pipeline parses, chunks, embeds, and stores them so 2.4 can retrieve them.

## Pipeline

```text
Markdown files
    ↓
Parser (goldmark)
    ↓
Chunking (heading-scoped, with a character budget)
    ↓
Metadata (source, file_path, heading, topic, status, created_at)
    ↓
Embeddings (llm.Provider.Embeddings — already implemented, zero callers today)
    ↓
knowledge_chunks (embedding as float32 BLOB)
```

## Supported Formats

- `.md` (primary)
- `.txt`

## Chunking

Heading-first with a character budget, not fixed-size: the mandated schema has a `heading` column, and pure fixed-size chunking would leave it empty.

- Parse with goldmark, walk `*ast.Heading` levels 1–3, read line offsets via `node.Lines()`, and **slice the raw markdown between headings**. Do not render to HTML — markdown reads better to the LLM and the implementation is ~40 lines instead of a renderer.
- `maxChunkChars = 2000`, `minChunkChars = 200`. **Token approximation without a tokenizer: 4 characters ≈ 1 token**, so ~500 tokens ≈ 2000 chars. Portuguese runs closer to 3.5 chars/token, which this budget absorbs conservatively.
- A section over budget is re-split on blank-line (paragraph) boundaries, inheriting its parent heading.
- A section under the minimum (a lone heading, a one-line stub) merges forward into the next, so the store does not fill with junk vectors.
- **No overlap.** Heading-scoped chunks are self-contained and overlap doubles embedding cost. Revisit only if retrieval quality proves poor.
- `.txt` skips goldmark: paragraph-split under the same budget, `Heading = ""`.

goldmark lives in the application layer — it is a library, not an adapter (`uuid` is already imported there).

## Domain

```go
type Chunk struct {
    ID        string
    Source    string // athena | user_note | imported_doc
    Topic     string
    Status    string
    ItemID    string // set only when Source == athena
    FilePath  string
    Heading   string
    Content   string
    Embedding []float32
    CreatedAt time.Time
}

type ChunkRepository interface {
    SaveAll(ctx context.Context, chunks []Chunk) error
    ListAll(ctx context.Context) ([]Chunk, error)
    DeleteByFilePath(ctx context.Context, path string) error
    DeleteByItemID(ctx context.Context, itemID string) error
}

type IngestedFileRepository interface {
    ListAll(ctx context.Context) (map[string]int64, error) // path -> mtime, one query per import
    Upsert(ctx context.Context, path string, mtime int64, chunkCount int) error
}
```

Imported notes get `Status = approved` — user-authored content is trusted by definition, so a single `Status: approved` filter in 2.4 covers both notes and athena items. `Topic` is the first-level directory under the picked root, falling back to the file's H1, falling back to the base name.

## Schema

```sql
CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id         TEXT PRIMARY KEY,
    source     TEXT,
    topic      TEXT,
    status     TEXT,
    item_id    TEXT,
    file_path  TEXT,
    heading    TEXT,
    content    TEXT,
    embedding  BLOB, -- tightly-packed little-endian float32
    created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_file_path ON knowledge_chunks(file_path);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_item_id  ON knowledge_chunks(item_id);

CREATE TABLE IF NOT EXISTS ingested_files (
    file_path   TEXT PRIMARY KEY,
    mtime       INTEGER NOT NULL,   -- Unix seconds
    chunk_count INTEGER NOT NULL,
    ingested_at DATETIME
);
```

Two documented extensions over the original schema:

- **`topic` / `status` / `item_id` on `knowledge_chunks`** — without them, 2.4's `SearchFilters` is unimplementable, and `item_id` is what lets 2.8 re-index or evict a chunk when its Knowledge Item is edited, deprecated, or deleted.
- **`ingested_files`** answers where the dedup mtime lives. A separate table rather than a column on `knowledge_chunks`: one indexed lookup instead of a scan, no mtime denormalized across N rows, and it records files that produced **zero** chunks (otherwise an empty file is re-read on every import).

## Embedding encoding

```go
// encodeEmbedding packs vec as tightly-packed little-endian IEEE-754
// binary32: 4 bytes per component, no header. Dimension is len(blob)/4.
func encodeEmbedding(vec []float32) []byte
func decodeEmbedding(blob []byte) ([]float32, error) // errors when len%4 != 0
```

- The **float64 → float32 conversion happens in `internal/application/ingest`**, where `llm.EmbeddingResponse.Embedding` becomes a `Chunk`. 1536 dims × 4 B = 6 KB per chunk, so 10k chunks is 61 MB resident instead of 123 MB; the ranking error from float32 rounding is ~1e-7.
- Little-endian because every supported target is LE, which keeps a copied `athena.db` portable.
- No version or dimension header. Consequence: **changing `llm.EmbeddingModel` requires a full re-ingest** — record it in ADR-004 and `README.md`.

## Filesystem access

Use `os.OpenRoot(root)` + `Root.FS()`. This gives symlink-escape confinement for free — the concrete answer to the path-traversal rule in `AGENTS.md` — and because the use case takes an `fs.FS`, tests drive it with `fstest.MapFS`: no new mock, no temp-dir fixtures. The desktop binding opens the root; the use case never touches `os`.

## Use case

```go
func (s *Service) ImportFolder(ctx context.Context, root string, onProgress func(Progress) error) (Summary, error)

type Progress struct{ FilesProcessed, FilesTotal, ChunksCreated int; CurrentFile string }
type Summary struct {
    FilesScanned, FilesIngested, FilesSkipped, FilesFailed, ChunksCreated int
    Failures []FileFailure // {Path, Reason}
}
```

1. Pre-walk collecting `.md`/`.txt` candidates (case-insensitive extension) to get a real `FilesTotal`
2. `ingestedFiles.ListAll` once → path→mtime map
3. Per file: unchanged mtime → count as skipped and continue. Otherwise read → chunk → embed → `chunks.DeleteByFilePath` (a changed file must **replace**, not duplicate) → `chunks.SaveAll` → `ingestedFiles.Upsert`
4. Any per-file error is recorded in the summary and the walk continues
5. `onProgress` after each file

Embedding calls stay **sequential** in this spec (100 files × ~5 chunks ≈ 500 calls ≈ 25 s behind a progress bar). Concurrency is a documented follow-up, not a first-commit optimization.

## Tasks

- [ ] `go.mod` — add `github.com/yuin/goldmark` (pure Go, no CGO)
- [ ] `internal/domain/knowledge/chunk.go` — `Chunk`, `ChunkRepository`, `IngestedFileRepository`
- [ ] `internal/infrastructure/sqlite/migrations.go` — `knowledge_chunks`, `ingested_files`, both indexes
- [ ] `internal/infrastructure/sqlite/embedding.go` — `encodeEmbedding` / `decodeEmbedding`
- [ ] `internal/infrastructure/sqlite/chunk_repository.go`, `ingested_file_repository.go`
- [ ] `internal/application/ingest/chunking.go` — pure chunker, no I/O
- [ ] `internal/application/ingest/service.go`, `import_folder.go` — pipeline over `fs.FS`
- [ ] `internal/interfaces/desktop/ingest.go` — `PickNotesFolder()` and `ImportNotes(path)` as **separate** bindings; events `ingest:progress` / `ingest:done` / `ingest:error` mirroring `study:*`
- [ ] `internal/interfaces/desktop/app.go` — `openDirectory` field defaulted to `wailsruntime.OpenDirectoryDialog`, injectable exactly like `emit`
- [ ] `frontend/src/lib/ingest.ts` — wrapper + `onIngestProgress` / `onIngestDone` / `onIngestError`
- [ ] "Import notes" button in the Knowledge Explorer toolbar + progress `Dialog` (vendor shadcn `progress`)

> Splitting the picker from the import is what makes `ImportNotes` testable against a `t.TempDir()`. The `openDirectory` field is not polish: the Wails runtime calls `log.Fatal` on a non-Wails context, which would `os.Exit` the test binary.

## Acceptance Criteria

- User picks a folder; every `.md` and `.txt` beneath it is ingested
- Each file is split into chunks; every chunk has an embedding stored in `knowledge_chunks`
- Re-importing the same folder is idempotent: the summary reports all files skipped and the chunk count is unchanged
- Editing one file and re-importing reprocesses only that file and does not duplicate its chunks
- A file that fails is reported in the summary and does not abort the import
- A folder with 100 markdown files completes without crashing, with progress reported per file
- `encodeEmbedding` / `decodeEmbedding` round-trip; an odd-length blob returns an error; byte order matches a hand-written expected `[]byte`
- A section exactly at `maxChunkChars` is kept whole; one over it is split on a paragraph boundary
- A section under `minChunkChars` is merged into the next rather than stored alone
