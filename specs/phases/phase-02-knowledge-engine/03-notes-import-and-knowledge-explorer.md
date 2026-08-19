# Phase 2.3 — Notes Import & Knowledge Explorer

> Supersedes the original, separately-tracked **2.3 — Notes Import (Markdown)** and
> **2.6 — Knowledge Explorer (UI)**. They shipped as one spec because the design
> turned out to be genuinely coupled: every imported note also becomes a
> `knowledge.Item`, so the screen that browses/manages Items necessarily has to
> exist for imported notes to be usable at all, and the screen's "Import notes"
> button has nowhere to live before the Knowledge nav section is unlocked. See
> "Why 2.3 and 2.6 merged" below.

## Goal

User imports a folder of personal notes; the pipeline parses, chunks, embeds, and
stores them so 2.4 can retrieve them — **and** the user can browse, filter, and
manage all knowledge content (Athena-extracted Items and imported notes alike)
from one dedicated screen, without needing to know or care which of the two
produced a given entry.

## Why 2.3 and 2.6 merged

The original two-track plan (`2.1 → 2.2 → 2.6 → 2.7` and `2.3 → 2.4 → 2.5 → 2.8`,
per `Planning.md`) assumed imported notes and Athena-extracted knowledge were
independent enough to ship on separate tracks: notes import wrote directly to
`knowledge_chunks` with no `knowledge.Item` involved, while the Explorer (2.6)
only ever listed `Item`s.

That produced a real gap: **imported notes would have been invisible in the
Explorer**, with no screen to browse or delete them, and no path back to
"I don't want this note anymore" other than editing the file that produced it.
Closing that gap without unifying the two data models would have meant the
Explorer maintaining two parallel listings (`Item`s and imported-file records)
with different action sets — exactly the inhomogeneous experience this spec's
design conversation set out to avoid.

The resolution: every imported file also produces exactly one lightweight,
heuristically-built `knowledge.Item` (no LLM call — see "Shadow Items" below).
Chunks stay the fidelity-preserving unit used for search (2.4/2.5); the Item is
purely a browsable/manageable representative. Once that Item exists, the
Explorer's existing `ListItems`/`ListTopics`/`DeleteItem` machinery works for
imported notes with **zero special-casing** — which is what makes shipping the
two specs together the smaller, not the larger, amount of work.

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
    +
Shadow knowledge_items entry (one per file, heuristic Concept/Definition, no LLM)
```

## Supported Formats

- `.md` (primary)
- `.txt`

## Chunking

Heading-first with a character budget, not fixed-size: the mandated schema has a
`heading` column, and pure fixed-size chunking would leave it empty.

- Parse with goldmark, walk `*ast.Heading` levels 1–3, read line offsets via
  `node.Lines()`, and **slice the raw markdown between headings**. Do not render
  to HTML — markdown reads better to the LLM and the implementation is ~40 lines
  instead of a renderer. Heading boundaries are **flat**: any level-1–3 heading
  starts a new section regardless of nesting, so `Heading` on a stored chunk is
  always the text of the single heading that opened its section — never a
  breadcrumb like `"Física > Cinemática"`.
- `maxChunkChars = 2000`, `minChunkChars = 200`. **Token approximation without a
  tokenizer: 4 characters ≈ 1 token**, so ~500 tokens ≈ 2000 chars. Portuguese
  runs closer to 3.5 chars/token, which this budget absorbs conservatively.
- A section over budget is re-split on blank-line (paragraph) boundaries,
  inheriting its parent heading.
- A section under the minimum (a lone heading, a one-line stub) merges **forward**
  into the next section, so the store does not fill with junk vectors. **The last
  section has no next**: it is merged **backwards** into its predecessor instead,
  and kept as-is if it is the only section. Content is never dropped for being
  short.

  Either direction, the resulting chunk's `Heading` is whichever of the two
  original headings **dominates the merged content by volume** — normally the
  non-stub side. The other heading's text is not lost; it stays inside `Content`,
  it simply is not promoted to the indexed `Heading` value. This keeps a single
  rule for both merge directions instead of one rule per direction.
- **A merge is allowed to push a chunk over `maxChunkChars`.** The budget is a
  target, not a hard ceiling; "content is never dropped for being short" outranks
  it. No re-split pass runs after a merge — the rare small overshoot (a section a
  little over 2000 chars) is accepted rather than adding a second splitting pass
  for an edge case that costs nothing in practice.
- **No content is lost for lacking a heading.** Text before the first H1–H3 (front
  matter, an intro paragraph) becomes its own leading chunk with `Heading = ""`. A
  document with no H1–H3 at all — plain prose, or one using only H4+ — falls back
  wholesale to the `.txt` path below: paragraph splitting under the same budget. A
  notes folder written as running text must ingest, not silently produce zero
  chunks.
- **No overlap.** Heading-scoped chunks are self-contained and overlap doubles
  embedding cost. Revisit only if retrieval quality proves poor.
- `.txt` skips goldmark: paragraph-split under the same budget, `Heading = ""`.

goldmark lives in the application layer — it is a library, not an adapter
(`uuid` is already imported there).

## Shadow Items

Every imported file also gets **exactly one** `knowledge.Item` — a lightweight,
non-LLM record that exists purely so the file has a first-class, manageable
representative in the Explorer. This is not extraction: nothing here calls the
LLM, and the actual searchable text stays in the raw chunks described above,
verbatim. Turning notes into LLM-summarized `Concept`/`Definition` pairs the way
2.2 does for chat sessions was considered and rejected — it would mean search
(2.4/2.5) running over an LLM's paraphrase of the user's own notes instead of
their actual words, which defeats the point of importing personal notes in the
first place. Cost was the other reason: an extraction call per file/section,
against ~500 embedding calls that already cover a 100-file import in the
budget below.

Granularity is **one Item per file**, not one per chunk or per heading section.
This matches every other file-scoped concept in this pipeline (`FilePath` is the
dedup key in `ingested_files`, the unit `DeleteByFilePath` operates on, and what
a user means by "that note"); a per-section Item would introduce a second,
finer-grained unit of management that nothing else in the pipeline recognizes,
undermining exactly the homogeneity this merge exists to deliver.

Fields, built purely from already-parsed data, no LLM call:

- `Concept` — the file's H1, falling back to the file's base name without
  extension. (Deliberately *not* the same fallback chain as `Topic` — `Topic` is
  the file's category/directory, `Concept` needs to identify the specific file.)
- `Definition` — the first 300 characters of the file's leading chunk (the one
  with `Heading = ""` if the file has front matter/intro text, otherwise its
  first real section), truncated on a word boundary with a trailing `…`. A plain
  textual preview, never a rewrite.
- `Properties`, `TradeOffs`, `RelatedConcepts` — left empty. These fields only
  make sense for LLM-structured content; inventing values for them here would be
  fabricating data that doesn't exist.
- `Source = SourceImportedDoc`, `Status = StatusApproved` set directly — no
  `draft` stage. User-authored content is trusted by definition (see "Domain"
  below), and a shadow Item was never extracted by anything fallible that would
  need a human check.
- `Topic` — same fallback chain as the chunks it stands in for (see "Domain").

**Stability across re-imports.** The Item's `ID` stays stable across
re-imports of the same file — chosen over "new Item every re-import" so that
selection/navigation state, and any future evidence/history links (2.9, 2.12),
don't reset every time a note is edited. The ID is carried in a new field on
`IngestedFile` (see "Domain"): recorded on first import, reused on every
subsequent one.

`Concept`/`Definition`/`Topic` are **always regenerated from the current file
content** on a re-import that changes anything — including overwriting a manual
edit the user made to those fields via the Explorer. This matches the "the file
is the source of truth" rule the rest of this pipeline already follows for
chunks (a changed file **replaces** its chunks, it does not merge with them);
introducing edit-preservation just for the shadow Item's title/preview would be
a special case the rest of the pipeline doesn't have.

On a re-import: `Save` a new `Item` (`uuid.NewString()`, `CreatedAt = UpdatedAt =
now`) the first time a file is seen; on every subsequent import of a changed
file, `GetByID` the existing Item (using the ID recorded in `IngestedFile`),
overwrite `Topic`/`Concept`/`Definition`/`UpdatedAt`, and `Update` — `ID`,
`Source`, `Status`, and `CreatedAt` are preserved from the existing record.

## Domain

```go
type Chunk struct {
    ID        string
    Source    string // athena | user_note | imported_doc
    Topic     string
    Status    string
    ItemID    string // the owning knowledge.Item — always set: the extracted Item
                      // for Source == athena, the shadow Item for imported_doc
    FilePath  string
    Heading   string
    Content   string
    Embedding []float32
    EmbeddingModel string
    ItemUpdatedAt time.Time // zero for imported files; detects stale Knowledge Item
                             // chunks after a 2.8 indexing failure. Imported-file
                             // chunks deliberately stay NULL here even though they
                             // now carry an ItemID — their dedup/staleness signal is
                             // ingested_files.mtime, a different mechanism serving a
                             // different purpose (see 2.8's own item_updated_at use)
    CreatedAt time.Time
}

type ChunkRepository interface {
    SaveAll(ctx context.Context, chunks []Chunk) error
    ListAll(ctx context.Context) ([]Chunk, error)
    DeleteByFilePath(ctx context.Context, path string) error
    DeleteByItemID(ctx context.Context, itemID string) error
}

type IngestedFileRepository interface {
    ListAll(ctx context.Context) (map[string]IngestedFile, error) // keyed by path, one query per import
    Upsert(ctx context.Context, file IngestedFile) error
}

type IngestedFile struct {
    Path           string
    MTime          int64
    EmbeddingModel string
    ChunkCount     int
    ItemID         string // the shadow Item's stable ID; carried across re-imports
}
```

`ItemID string // set only when Source == athena` from the original 2.3 draft is
**no longer accurate** and is corrected above: every chunk now has an owning
Item, Athena-extracted or shadow.

Imported notes get `Status = approved` — user-authored content is trusted by
definition, so a single `Status: approved` filter in 2.4 covers both notes and
athena items. `Topic` is the first-level directory under the picked root,
falling back to the file's H1, falling back to the base name — used both for the
chunk's `Topic` and the shadow Item's `Topic`, so the two line up under the same
tree entry in the Explorer.

**Deleting an imported note's Item does not delete its file, and does not
un-suppress it on the next import.** `DeleteItem` (below) cascades to
`ChunkRepository.DeleteByItemID`, removing the Item and its chunks — but it never
touches the `ingested_files` row for that path, and that omission is
deliberate, not an oversight: `ingested_files` dedup is mtime-based (see
"Schema"), so as long as that row still records the file's current mtime, an
unedited file is skipped on the next import and the deleted content **stays
deleted**. If the row were cleared on delete, the very next import of the same
folder would silently resurrect content the user just chose to remove. The
file itself is never touched by this pipeline in either direction — deleting a
note here has zero effect on the user's filesystem, and deleting the file on
disk has zero effect on already-imported content (see "Filesystem access").

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
    embedding_model TEXT NOT NULL,
    item_updated_at DATETIME, -- NULL for imported_doc; see Domain above
    created_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_file_path ON knowledge_chunks(file_path);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_item_id  ON knowledge_chunks(item_id);

CREATE TABLE IF NOT EXISTS ingested_files (
    file_path       TEXT PRIMARY KEY,
    mtime           INTEGER NOT NULL,   -- Unix seconds
    embedding_model TEXT NOT NULL,      -- vectors are only reusable for the model that produced them
    chunk_count     INTEGER NOT NULL,
    item_id         TEXT NOT NULL,      -- the shadow Item's stable ID; new column vs. the original draft
    ingested_at     DATETIME
);
```

`knowledge_items` is unchanged from 2.1 — shadow Items are ordinary rows in that
same table, distinguished only by `source = 'imported_doc'`.

Documented extensions over the pre-merge schema:

- **`topic` / `status` / `item_id` on `knowledge_chunks`** — without them, 2.4's
  `SearchFilters` is unimplementable, and `item_id` is what lets 2.8 re-index or
  evict a chunk when its Knowledge Item is edited, deprecated, or deleted — and,
  as of this merge, what lets `DeleteItem` remove an imported note's chunks with
  no imported-note-specific code path.
- **`embedding_model` / `item_updated_at` on `knowledge_chunks`** — vectors from
  different models must never be mixed even when their dimensions happen to
  match; 2.8 also needs to distinguish a present-but-stale item chunk from a
  current one after an indexing failure. Imported-file chunks leave
  `item_updated_at` NULL and continue using `ingested_files` for mtime
  deduplication, independent of the fact that they now carry an `item_id`.
- **`ingested_files`** answers where the dedup mtime lives. A separate table
  rather than a column on `knowledge_chunks`: one indexed lookup instead of a
  scan, no mtime denormalized across N rows, and it records files that produced
  **zero** chunks (otherwise an empty file is re-read on every import).
  `item_id` is the new column this merge adds — see "Shadow Items" above for why
  it needs to survive across re-imports and deletes.

  `embedding_model` is part of the dedup key: a file is skipped only when
  **both** its mtime and the current `llm.EmbeddingModel` match what is
  recorded. Vectors are only comparable to others from the same model, so
  changing the model must re-embed rather than leave a silently mixed store —
  this turns the "changing the model requires a re-ingest" consequence below
  from a README footnote into automatic behavior.

  Dedup remains **mtime-based, not content-hash-based**: hashing would mean
  reading and digesting every file on every import, which is exactly the cost
  mtime dedup exists to avoid. The gap this leaves — content changed while mtime
  was preserved, as `git checkout` and `rsync -t` can do — is accepted, with a
  full re-import as the manual escape hatch.

## Embedding encoding

```go
// encodeEmbedding packs vec as tightly-packed little-endian IEEE-754
// binary32: 4 bytes per component, no header. Dimension is len(blob)/4.
func encodeEmbedding(vec []float32) []byte
func decodeEmbedding(blob []byte) ([]float32, error) // errors when len%4 != 0
```

- The **float64 → float32 conversion happens in `internal/application/ingest`**,
  where `llm.EmbeddingResponse.Embedding` becomes a `Chunk`. 1536 dims × 4 B = 6
  KB per chunk, so 10k chunks is 61 MB resident instead of 123 MB; the ranking
  error from float32 rounding is ~1e-7.
- Little-endian because every supported target is LE, which keeps a copied
  `athena.db` portable.
- No version or dimension header. Consequence: **changing `llm.EmbeddingModel`
  requires a full re-ingest** — record it in ADR-004 and `README.md`.

## Filesystem access

Use `os.OpenRoot(root)` + `Root.FS()`. This gives symlink-escape confinement for
free — the concrete answer to the path-traversal rule in `AGENTS.md` — and
because the use case takes an `fs.FS`, tests drive it with `fstest.MapFS`: no new
mock, no temp-dir fixtures. The desktop binding opens the root; the use case
never touches `os`.

The pre-walk (see "Use case") **skips directories whose name starts with `.`**
entirely (`fs.SkipDir` on a matching `DirEntry`) — `.git`, `.obsidian`, and
similar tooling directories never contribute candidates and, for `.git`
specifically, can otherwise make the walk needlessly slow on a notes folder
that is also a repository. No acceptance criterion depends on hidden files being
processed, so this costs nothing in correctness.

Import is one-directional and disk state is **not** the source of truth once a
file has been ingested: deleting a file from disk after import has no effect on
its already-stored chunks or shadow Item (see "Domain" above) — there is no
detection of files removed from the folder, and none is planned. If a user no
longer wants a note's content indexed, they remove it from the Explorer (see
"Actions" below), not from their filesystem; the filesystem and the knowledge
store are treated as decoupled once import has happened once.

## Use case — import pipeline

```go
func (s *Service) ImportFolder(ctx context.Context, root string, onProgress func(Progress) error) (Summary, error)

type Progress struct{ FilesProcessed, FilesTotal, ChunksCreated int; CurrentFile string }
type Summary struct {
    FilesScanned, FilesIngested, FilesSkipped, FilesFailed, ChunksCreated int
    Failures []FileFailure // {Path, Reason}
}
```

`Service` gains a `items domainknowledge.Repository` dependency (aliased the same
way 2.2 already does, to avoid the `internal/application/knowledge` /
`internal/domain/knowledge` package-name collision) alongside the
`ChunkRepository` and `IngestedFileRepository` from the original draft. There is
**no `VectorStore` dependency** — that type is introduced by 2.4/2.8, not this
spec; this pipeline's job ends at persisting to SQLite. 2.4 rebuilds its
in-memory index from `knowledge_chunks` at startup, so the table alone is a
complete, correct source of truth the moment this transaction commits.

1. Pre-walk collecting `.md`/`.txt` candidates (case-insensitive extension,
   hidden directories skipped) to get a real `FilesTotal`
2. `ingestedFiles.ListAll` once → path→`IngestedFile` map
3. Per file: unchanged mtime **and** unchanged embedding model → count as
   skipped and continue. Otherwise read → chunk → embed → build the shadow Item
   fields → replace.

   Embedding happens **before** anything is deleted, so a failed or interrupted
   API call leaves the previous chunks and Item intact. The replace itself —
   `chunks.DeleteByFilePath` (a changed file must **replace**, not duplicate) →
   `chunks.SaveAll` → `items.Save` (first import) or `items.GetByID` +
   `items.Update` (re-import, existing `ItemID` from the map) →
   `ingestedFiles.Upsert` (now carrying `ItemID`) — runs inside a **single
   SQLite transaction**, so a failure cannot leave any of these four writes
   applied without the others. `MaxOpenConns(1)` makes this free, and is why
   extending the transaction to cover `items` costs nothing beyond the original
   3-repository version.
4. Any per-file error is recorded in the summary and the walk continues
5. `onProgress` is called after each file (processed or skipped). **If it
   returns a non-nil error, `ImportFolder` stops the walk immediately** and
   returns the `Summary` accumulated so far alongside that error — a callback
   that can never actually stop anything would have no reason to return
   `error` in the first place, and this is the hook a future cancel affordance
   (out of scope here — see "UI: progress" below) would use.

Embedding calls stay **sequential** in this spec (100 files × ~5 chunks ≈ 500
calls ≈ 25 s behind a progress bar). Concurrency is a documented follow-up, not
a first-commit optimization.

## Explorer UI

### Layout

The `knowledge` nav section already exists in `frontend/src/lib/navigation.ts` as
`{phase: 2, status: 'locked'}` — this spec unlocks it.

```text
sidebar rail                     main pane
─────────────────────            ────────────────────────────────────────
  Knowledge  ⟨3⟩                   [ Explorer | Review ⟨3⟩ ]  [Import notes]
    ├ All topics                   Status: ⟨All ▾⟩
    ├ Go                           ┌───────────────┬──────────────────────┐
    ├ Kubernetes                   │ item list     │ detail + actions     │
    └ System Design                └───────────────┴──────────────────────┘
```

- The topic tree renders inside the sidebar rail under the Knowledge item,
  exactly where `StudyFolderTree` renders under Study in `app-shell.tsx`.
- The main pane splits with plain flex (`w-80 shrink-0 border-r` list column +
  `flex-1` detail column). **Do not nest a second `ResizablePanelGroup`** inside
  the existing one.
- Explorer/Review is a local `'explorer' | 'review'` state machine, consistent
  with `App.tsx` and `app-shell.tsx` — the project deliberately has no router.
- **No dual listing.** Because every imported file is a shadow `Item`, the topic
  tree and item list are driven by the same `ListItems`/`ListTopics` calls used
  for Athena-extracted content — there is no second query path, no merge logic,
  and no separate "imported files" section. Each row carries a `Source` `Badge`
  ("Athena" / "Imported note") so provenance stays visible without requiring a
  separate view; two entries under the same topic, one of each source, is the
  expected way a user sees "I imported a note on this and also discussed it with
  Athena" (the scenario that originally motivated unifying the two specs).
- **"Import notes"** sits in the toolbar next to the Explorer/Review tabs (per
  the layout above) and opens the OS folder picker (`PickNotesFolder`,
  desktop-bound). On a folder being chosen, `ImportNotes(path)` runs and a
  progress `Dialog` (vendored shadcn `progress`) takes over, showing
  `FilesProcessed / FilesTotal` and the current file name, driven by
  `ingest:progress` / `ingest:done` / `ingest:error` events mirroring the
  existing `study:*` pattern.

  There is **no cancel affordance** on this dialog in this phase — no operation
  in the app is cancellable today (confirmed: not even the longer-running study
  streaming calls), and adding the first one here would be new, unscoped
  infrastructure. Each file's replace is already an isolated transaction, so
  worst case the user waits out the ~25 s/100-file estimate.

  On `ingest:done`, the dialog **replaces the progress bar with a result
  summary** inside the same dialog — counts (`scanned` / `ingested` / `skipped`
  / `failed`) plus the `Failures` list with each path and reason — and requires
  a manual "Close" to dismiss. Per-file failures are actionable
  information (the user may need to fix and re-import a specific file); a
  toast that disappears in a few seconds risks a real failure going unnoticed,
  and the project has no toast/notification system today to reuse anyway.

### Actions

Actions are gated by the domain lifecycle, so the UI never offers an illegal
transition:

| Status | Available actions |
|---|---|
| `draft` | Approve · Edit · Delete |
| `approved` | Deprecate · Edit · Delete |
| `deprecated` | Edit · Delete |

This table is unchanged by the merge and needs no imported-note-specific row:
shadow Items are created directly at `approved` (see "Shadow Items"), so
`draft`'s "Approve" is simply never offered to one, exactly as the table
already dictates for any already-approved item.

Delete is irreversible and sits behind an `AlertDialog`, mirroring the
folder-delete flow in `study-folder-tree.tsx`. **When the Item being deleted has
`Source = imported_doc`, the dialog's copy gains an extra sentence** explaining
the consequence unique to that source: deleting here does not delete the
original file, and re-importing the same folder later will not bring this
content back unless the file is edited first (see "Domain" above for why). The
copy does not name the specific file — `knowledge.Item` deliberately carries no
file path (`Source` is a category, not provenance; see 2.1 and 2.9), so this
stays a general explanation of the behavior rather than a per-file detail.

## Note-template guidance (out of scope)

A button offering a note-writing template, or app-side reformatting of an
unstructured note before ingest, was considered during this spec's design and
explicitly deferred: it does not depend on anything decided above and is pure
UX guidance layered on top of an already-larger-than-planned merge. Track it as
a follow-up, not a task below.

## Tasks

- [ ] `go.mod` — add `github.com/yuin/goldmark` (pure Go, no CGO)
- [ ] `internal/domain/knowledge/chunk.go` — `Chunk` (with the corrected `ItemID`
      comment), `ChunkRepository`, `IngestedFileRepository`, `IngestedFile` (with
      the new `ItemID` field)
- [ ] `internal/infrastructure/sqlite/migrations.go` — `knowledge_chunks`,
      `ingested_files` (with `item_id`), both indexes
- [ ] `internal/infrastructure/sqlite/embedding.go` — `encodeEmbedding` /
      `decodeEmbedding`
- [ ] `internal/infrastructure/sqlite/chunk_repository.go`,
      `ingested_file_repository.go`
- [ ] `internal/application/ingest/chunking.go` — pure chunker, no I/O
- [ ] `internal/application/ingest/shadow_item.go` — builds `Concept`/
      `Definition`/`Topic` for a file's shadow Item from already-parsed chunk
      data; pure function, no I/O
- [ ] `internal/application/ingest/service.go`, `import_folder.go` — pipeline
      over `fs.FS`; `Service` takes `chunks ChunkRepository`, `ingestedFiles
      IngestedFileRepository`, `items domainknowledge.Repository`, `llm
      domainllm.Provider`
- [ ] `internal/interfaces/desktop/ingest.go` — `PickNotesFolder()` and
      `ImportNotes(path)` as **separate** bindings; events `ingest:progress` /
      `ingest:done` / `ingest:error` mirroring `study:*`
- [ ] `internal/interfaces/desktop/app.go` — `openDirectory` field defaulted to
      `wailsruntime.OpenDirectoryDialog`, injectable exactly like `emit`
- [ ] `frontend/src/lib/ingest.ts` — wrapper + `onIngestProgress` /
      `onIngestDone` / `onIngestError`
- [ ] `internal/application/knowledge/list.go` — `ListItems(ctx, topic,
      status)`, `ListTopics(ctx)` — unchanged by the merge; already covers
      shadow Items with no special-casing
- [ ] `internal/application/knowledge/approve.go` — `Approve(ctx, id)`: load →
      `TransitionTo` → `Update`. **This is the seam 2.8's indexing hook plugs
      into**
- [ ] `internal/application/knowledge/deprecate.go` — `Deprecate(ctx, id)`
- [ ] `internal/application/knowledge/update.go` — `UpdateItem(ctx, id,
      fields)`: validates, restamps `UpdatedAt`, never touches `Status` /
      `Source` / `CreatedAt`
- [ ] `internal/application/knowledge/delete.go` — `DeleteItem(ctx, id)`;
      cascades to `chunks.DeleteByItemID`, never touches `ingested_files`
- [ ] `internal/interfaces/desktop/knowledge.go` — bindings with these exact
      returns:

  | Binding | Returns |
  |---|---|
  | `ListKnowledgeItems(topic, status)` | `([]KnowledgeItemResult, error)` |
  | `ListKnowledgeTopics()` | `([]string, error)` |
  | `ApproveKnowledgeItem(id)` | `(KnowledgeItemResult, error)` |
  | `DeprecateKnowledgeItem(id)` | `(KnowledgeItemResult, error)` |
  | `UpdateKnowledgeItem(id, input)` | `(KnowledgeItemResult, error)` |
  | `DeleteKnowledgeItem(id)` | `error` |

  Approve, Deprecate, and Update return the updated item so React can patch
  local state without a refetch (precedent: `UpdateProfile` in `settings.go`).
  Delete returns only an error — there is no item left to return.
  `KnowledgeItemResult` includes `Source` so the frontend can render the
  provenance `Badge`.
- [ ] `frontend/src/lib/navigation.ts` — flip `knowledge` to `status:
      'unlocked'` (updates one assertion in `navigation.test.ts`)
- [ ] `frontend/src/lib/knowledge.ts` — add the new wrappers plus pure helpers
      `groupByTopic(items)` and `definitionPreview(text, max)` (the latter
      already works unmodified for shadow Items — `Definition` is plain text
      for both sources)
- [ ] `frontend/src/components/knowledge-section.tsx` — Explorer/Review tab
      state; toolbar includes the **"Import notes"** button
- [ ] `frontend/src/components/knowledge-topic-tree.tsx` — topics from
      `ListKnowledgeTopics` plus an "All topics" row
- [ ] `frontend/src/screens/KnowledgeExplorerScreen.tsx` — status filter
      (`Select`), item list (concept + definition preview + status `Badge` +
      source `Badge`), detail view with all fields and the gated actions
- [ ] `frontend/src/components/knowledge-delete-dialog.tsx` (or inlined
      `AlertDialog` in the screen) — confirmation copy branches on `Source ===
      'imported_doc'` per "Actions" above
- [ ] Inline editor reusing `frontend/src/components/tag-input.tsx` for
      `Properties` / `TradeOffs` / `RelatedConcepts`, `Input` for concept,
      `Textarea` for definition
- [ ] `frontend/src/components/knowledge-extraction-dialog.tsx` — add the third
      button **[Save & approve]**, completing the flow of `specs/Athena.md` §12
- [ ] `frontend/src/components/ingest-progress-dialog.tsx` — progress bar
      (vendor shadcn `progress`) that transitions in place to the result
      summary on `ingest:done` / `ingest:error`

> Pushing logic (`groupByTopic`, `definitionPreview`, filter predicates) into
> `lib/knowledge.ts` as pure functions is what keeps the frontend 80% coverage
> and Stryker 80 thresholds reachable on two large screens — far better than
> letting Stryker chew on deep JSX branches.
>
> Splitting the picker from the import is what makes `ImportNotes` testable
> against a `t.TempDir()`. The `openDirectory` field is not polish: the Wails
> runtime calls `log.Fatal` on a non-Wails context, which would `os.Exit` the
> test binary.

## Acceptance Criteria

Pipeline:

- User picks a folder; every `.md` and `.txt` beneath it is ingested; hidden
  directories are skipped entirely
- Each file is split into chunks; every chunk has an embedding stored in
  `knowledge_chunks`, and exactly one shadow `knowledge.Item` is created for the
  file
- Re-importing the same folder is idempotent: the summary reports all files
  skipped and both the chunk count and the Item count are unchanged
- Editing one file and re-importing reprocesses only that file, does not
  duplicate its chunks, and updates its existing shadow Item in place (same
  `ID`, refreshed `Concept`/`Definition`/`Topic`/`UpdatedAt`) rather than
  creating a second one
- A file that fails is reported in the summary and does not abort the import
- An `onProgress` callback that returns an error stops the import immediately;
  the returned `Summary` reflects only the files processed before the stop
- A folder with 100 markdown files completes without crashing, with progress
  reported per file
- `encodeEmbedding` / `decodeEmbedding` round-trip; byte order matches a
  hand-written expected `[]byte`
- A blob whose length is not a multiple of four returns an error — asserted for
  both an odd length and an **even** invalid one (6 bytes), since the rule is
  `len%4`, not parity
- A section exactly at `maxChunkChars` is kept whole; one over it is split on a
  paragraph boundary
- A section under `minChunkChars` is merged into a neighbour rather than stored
  alone, keeping the dominant heading's text as the merged chunk's `Heading`; a
  short **final** section is merged backwards instead of dropped
- A merge that pushes the resulting chunk over `maxChunkChars` is accepted, not
  re-split
- A markdown file with no H1–H3 heading is still ingested via the
  paragraph-splitting fallback; text before the first heading becomes a chunk
  with an empty heading
- Changing `llm.EmbeddingModel` makes a previously ingested file re-embed
  instead of being skipped
- Every stored chunk records the embedding model that created its vector
- Deleting a file from disk after import has no effect on its already-stored
  chunks or Item; nothing in `ImportFolder` detects or reacts to files that
  disappear from the picked folder

Explorer:

- All Knowledge Items are visible in the explorer after a study session with
  extraction — **and** after a notes import, in the same list, distinguished by
  a `Source` badge
- Filtering by "draft" shows only unreviewed items (imported notes, being
  `approved` from creation, never appear there); selecting a topic in the tree
  restricts the list to that topic across both sources
- Clicking "Approve" changes the badge from "draft" to "approved" without a
  page reload
- "Approve" is not offered on an approved or deprecated item; "Deprecate" is
  offered only on approved items — including imported notes, which are
  `approved` immediately and so can be deprecated like any other approved item
- Editing a field and saving persists the change to SQLite and leaves `Status`,
  `Source`, and `CreatedAt` untouched
- Delete asks for confirmation and removes the item and its chunks permanently;
  for an imported note, the confirmation copy explains that the source file is
  untouched and that an unchanged file will not be re-imported automatically
- Deleting an imported note's Item leaves its `ingested_files` row intact, so
  re-importing the same unedited folder does not resurrect the deleted content;
  editing the source file and re-importing does bring it back (as a fresh
  replace, per the pipeline criteria above)
- The Knowledge nav section is unlocked and no longer renders `ComingSoonPanel`
- "Import notes" opens the folder picker and the import runs behind a progress
  dialog reporting per-file progress, ending in an in-dialog result summary
  (counts + failures) that requires manual dismissal
- After 2.9, the detail view exposes the immutable evidence supporting the item
- After 2.12, the detail view exposes its read-only revision timeline and field
  diffs